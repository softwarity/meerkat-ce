package auth

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/softwarity/meerkat/internal/store"
)

var mailedCode = regexp.MustCompile(`(?m)^\s+(\d{6})\s*$`)

// send is do with a Host of the caller's choosing and any number of cookies:
// the two things an attacker writes and a browser carries.
func send(t *testing.T, mux http.Handler, method, host, path string, form url.Values, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, path, body)
	if host != "" {
		req.Host = host
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for _, c := range cookies {
		if c != nil {
			req.AddCookie(c)
		}
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func codeCookieOf(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == mailCodeCookie && c.Value != "" {
			return c
		}
	}
	t.Fatalf("no %s cookie was set", mailCodeCookie)
	return nil
}

func codeIn(t *testing.T, text string) string {
	t.Helper()
	m := mailedCode.FindStringSubmatch(text)
	if m == nil {
		t.Fatalf("no code in the mail:\n%s", text)
	}
	return m[1]
}

func aliceWithPassword(t *testing.T, st *store.Store, password string) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser(context.Background(), store.User{
		ID: "alice", Username: "alice", PasswordHash: string(hash),
		Email: "alice@example.org", Enabled: true, EmailVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
}

// THE ATTACK (P0-01 of the 2026-10-12 audit): somebody asks a reset for
// another person's address with a Host of their own. The mail used to carry
// the real token on a link to that host. Now a link only points at a name a
// placed certificate covers, reached on the gateway's own TLS; anything else
// mails a code, and no link at all.
func TestAMailedLinkNeverPointsAtTheCallersHost(t *testing.T) {
	for _, c := range []struct {
		name    string
		overTLS bool
		host    string
		link    string // "" = the mail must carry a code
	}{
		{"plain http, a foreign host", false, "attacker.example", ""},
		{"plain http, our own name", false, "example.com", ""},
		{"our TLS, a name no certificate covers", true, "attacker.example", ""},
		{"our TLS, a covered name", true, "example.com", "https://example.com/reset-password?token="},
		// A name declared reached through a proxy that terminates TLS: the
		// request arrives in clear, and the link is still https, to that name.
		{"plain http, a declared proxy name", false, "shop.example.org", "https://shop.example.org/reset-password?token="},
		{"plain http, a declared proxy name in capitals", false, "SHOP.example.org", "https://SHOP.example.org/reset-password?token="},
		{"plain http, a declared proxy name and its port", false, "shop.example.org:8443", "https://shop.example.org:8443/reset-password?token="},
		{"plain http, one label under a declared wildcard", false, "eu.apps.example.org", "https://eu.apps.example.org/reset-password?token="},
		{"plain http, two labels under a declared wildcard", false, "a.eu.apps.example.org", ""},
		{"plain http, the wildcard's own domain", false, "apps.example.org", ""},
		{"plain http, a name beside a declared one", false, "example.org", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			mux, _, st, box := registerSetupOn(t, c.overTLS)
			aliceWithPassword(t, st, "old-pass-1234")
			rec := send(t, mux, "POST", c.host, "/forgot-password", url.Values{"email": {"alice@example.org"}})
			if rec.Code != http.StatusOK {
				t.Fatalf("forgot: %d", rec.Code)
			}
			mails := box.forRecipient("alice@example.org")
			if len(mails) != 1 {
				t.Fatalf("mails: %d, want 1", len(mails))
			}
			text := mails[0].Text + mails[0].HTML
			if strings.Contains(text, "attacker.example") {
				t.Fatalf("the mail points at the caller's host:\n%s", mails[0].Text)
			}
			if c.link == "" {
				if strings.Contains(text, "token=") {
					t.Fatalf("a mail that cannot vouch for its address carries a link:\n%s", mails[0].Text)
				}
				codeIn(t, mails[0].Text)
				if !strings.Contains(bodyString(rec), `action="/reset-password/code"`) {
					t.Fatalf("the page does not wait for the code")
				}
				return
			}
			if !strings.Contains(mails[0].Text, c.link) {
				t.Fatalf("want a link to %s in:\n%s", c.link, mails[0].Text)
			}
		})
	}
}

// The code opens the new-password form in the browser that asked, and
// nowhere else; the form then works as the link's does.
func TestAResetByCodeWorksInTheBrowserThatAsked(t *testing.T) {
	mux, _, st, box := registerSetup(t)
	aliceWithPassword(t, st, "old-pass-1234")
	asked := send(t, mux, "POST", "", "/forgot-password", url.Values{"email": {"alice@example.org"}})
	browser := codeCookieOf(t, asked)
	code := codeIn(t, box.forRecipient("alice@example.org")[0].Text)

	// Another browser - the attacker's own, which asked for nothing it can
	// read - gets nowhere with the right code.
	other := codeCookieOf(t, send(t, mux, "POST", "", "/forgot-password", url.Values{"email": {"ghost@example.org"}}))
	if rec := send(t, mux, "POST", "", "/reset-password/code", url.Values{"code": {code}}, other); rec.Code != http.StatusUnauthorized {
		t.Fatalf("the code worked in another browser: %d", rec.Code)
	}
	if rec := send(t, mux, "POST", "", "/reset-password/code", url.Values{"code": {code}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("no browser binding should start again: %d", rec.Code)
	}

	// One read: a recorder's body is a reader, and a second read is empty.
	tokenField := regexp.MustCompile(`name="token" value="([^"]+)"`)
	form := send(t, mux, "POST", "", "/reset-password/code", url.Values{"code": {code}}, browser)
	tok := tokenField.FindStringSubmatch(bodyString(form))
	if form.Code != http.StatusOK || tok == nil {
		t.Fatalf("the code did not open the new-password form: %d", form.Code)
	}
	if again := send(t, mux, "POST", "", "/reset-password/code", url.Values{"code": {code}}, browser); tokenField.MatchString(bodyString(again)) {
		t.Fatalf("a code opened the form twice")
	}
	done := send(t, mux, "POST", "", "/reset-password",
		url.Values{"token": {tok[1]}, "password": {"brand-new-pass-1"}, "confirm": {"brand-new-pass-1"}})
	if done.Code != http.StatusOK {
		t.Fatalf("reset: %d", done.Code)
	}
	if login := do(t, mux, "POST", "/login", url.Values{"username": {"alice"}, "password": {"brand-new-pass-1"}}, nil); login.Code != http.StatusSeeOther {
		t.Fatalf("the new password does not sign in: %d", login.Code)
	}
}

// A few wrong codes and the request is dead, as with the e-mail sign-in.
func TestAResetCodeCannotBeGuessed(t *testing.T) {
	mux, _, st, _ := registerSetup(t)
	aliceWithPassword(t, st, "old-pass-1234")
	browser := codeCookieOf(t, send(t, mux, "POST", "", "/forgot-password", url.Values{"email": {"alice@example.org"}}))
	for i := 0; i < signinMaxTries; i++ {
		if rec := send(t, mux, "POST", "", "/reset-password/code", url.Values{"code": {"000000"}}, browser); rec.Code != http.StatusUnauthorized {
			t.Fatalf("wrong code #%d: %d", i+1, rec.Code)
		}
	}
	if rec := send(t, mux, "POST", "", "/reset-password/code", url.Values{"code": {"000000"}}, browser); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("past the tries: %d", rec.Code)
	}
}

// Registering with somebody else's address and a host of one's own used to
// get their confirmation link; now the confirmation is a code, typed in the
// browser that registered.
func TestARegistrationIsConfirmedByCode(t *testing.T) {
	mux, _, st, box := registerSetup(t)
	form := url.Values{
		"username": {"bob"}, "email": {"bob@example.org"},
		"password": {"s3cret-s3cret"}, "confirm": {"s3cret-s3cret"},
	}
	rec := send(t, mux, "POST", "attacker.example", "/register", form)
	if rec.Code != http.StatusOK || !strings.Contains(bodyString(rec), `action="/confirm/code"`) {
		t.Fatalf("register: %d", rec.Code)
	}
	browser := codeCookieOf(t, rec)
	mail := box.forRecipient("bob@example.org")[0]
	if strings.Contains(mail.Text+mail.HTML, "attacker.example") || strings.Contains(mail.Text, "token=") {
		t.Fatalf("the confirmation carries a link:\n%s", mail.Text)
	}
	done := send(t, mux, "POST", "", "/confirm/code", url.Values{"code": {codeIn(t, mail.Text)}}, browser)
	if done.Code != http.StatusOK || !strings.Contains(bodyString(done), "confirmed") {
		t.Fatalf("confirm by code: %d", done.Code)
	}
	id, err := st.UserIDByEmail(context.Background(), "bob@example.org")
	if err != nil {
		t.Fatal(err)
	}
	if u, _ := st.GetUserByID(context.Background(), id); !u.EmailVerified {
		t.Fatalf("the address is not marked confirmed")
	}
}

// A new address is proved by a code typed in the session that asked for it.
func TestAnAddressChangeIsConfirmedByCode(t *testing.T) {
	mux, _, st, box := registerSetup(t)
	aliceWithPassword(t, st, "pass-1234-x")
	sess := sessionCookieOf(do(t, mux, "POST", "/login", url.Values{"username": {"alice"}, "password": {"pass-1234-x"}}, nil))

	asked := send(t, mux, "POST", "", "/profile/email", url.Values{"email": {"alice@new.example"}}, sess)
	if asked.Code != http.StatusOK || !strings.Contains(bodyString(asked), `action="/profile/email/code"`) {
		t.Fatalf("change: %d", asked.Code)
	}
	browser := codeCookieOf(t, asked)
	code := codeIn(t, box.forRecipient("alice@new.example")[0].Text)
	if u, _ := st.GetUserByID(context.Background(), "alice"); u.Email != "alice@example.org" {
		t.Fatalf("the address changed before it was proved")
	}
	// The code without the session that asked opens nothing.
	if rec := send(t, mux, "POST", "", "/profile/email/code", url.Values{"code": {code}}, browser); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Fatalf("no session: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	done := send(t, mux, "POST", "", "/profile/email/code", url.Values{"code": {code}}, sess, browser)
	if done.Code != http.StatusSeeOther || done.Header().Get("Location") != "/profile?email=confirmed" {
		t.Fatalf("confirm by code: %d %s", done.Code, done.Header().Get("Location"))
	}
	if u, _ := st.GetUserByID(context.Background(), "alice"); u.Email != "alice@new.example" {
		t.Fatalf("the address did not change: %s", u.Email)
	}
}
