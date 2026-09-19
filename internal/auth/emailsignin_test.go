package auth

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/mail"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// signinOn opens the door: the setting and a relay, which is what the flow
// asks for before it shows anything.
func signinOn(t *testing.T, st *store.Store) {
	t.Helper()
	if err := st.SetSetting(context.Background(), store.SettingEmailSignin, true); err != nil {
		t.Fatal(err)
	}
}

// askCode posts the address and returns the browser cookie the answer set -
// the one the code is bound to.
func askCode(t *testing.T, mux *http.ServeMux, email string) *http.Cookie {
	t.Helper()
	rec := do(t, mux, "POST", "/login/code", url.Values{"email": {email}, "next": {"/app"}}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("asking for a code answered %d", rec.Code)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == signinCookie && c.Value != "" {
			return c
		}
	}
	t.Fatalf("no %s cookie was set", signinCookie)
	return nil
}

// Off by default: no link on the sign-in page, and the three endpoints are not
// there at all. A control that appears and then refuses is worse than absent.
func TestEmailSigninClosedUntilEnabled(t *testing.T) {
	mux, _, sent := otpSetup(t)

	page := do(t, mux, "GET", "/login", nil, nil)
	if strings.Contains(bodyString(page), "/login/code") {
		t.Fatalf("the mailed-code link showed while the option was off")
	}
	for _, c := range []struct {
		method, path string
	}{{"GET", "/login/code"}, {"POST", "/login/code"}, {"POST", "/login/code/verify"}} {
		if rec := do(t, mux, c.method, c.path, url.Values{"email": {"admin@example.com"}}, nil); rec.Code != http.StatusNotFound {
			t.Fatalf("%s %s answered %d while off, want 404", c.method, c.path, rec.Code)
		}
	}
	if len(*sent) != 0 {
		t.Fatalf("a code was mailed while the option was off")
	}
}

// On, with a relay: the link shows, a code is mailed, and typing it back signs
// in - no password anywhere in the exchange.
func TestEmailSigninSignsIn(t *testing.T) {
	mux, st, sent := otpSetup(t)
	signinOn(t, st)

	page := do(t, mux, "GET", "/login", nil, nil)
	if !strings.Contains(bodyString(page), "/login/code") {
		t.Fatalf("the mailed-code link is missing when it should show")
	}

	cookie := askCode(t, mux, "admin@example.com")
	if len(*sent) != 1 {
		t.Fatalf("want one mail, got %d", len(*sent))
	}
	code := digitsOf(t, (*sent)[0].Text)

	bad := do(t, mux, "POST", "/login/code/verify", url.Values{"code": {"000000"}, "next": {"/app"}}, cookie)
	if bad.Code != http.StatusUnauthorized {
		t.Fatalf("a wrong code answered %d, want 401", bad.Code)
	}
	ok := do(t, mux, "POST", "/login/code/verify", url.Values{"code": {code}, "next": {"/app"}}, cookie)
	if ok.Code != http.StatusSeeOther || ok.Header().Get("Location") != "/app" {
		t.Fatalf("the mailed code did not sign in: %d %q", ok.Code, ok.Header().Get("Location"))
	}
}

// An address nobody uses gets the same page, the same status and a cookie all
// the same: the answer may not say whether an account is behind it.
func TestEmailSigninDoesNotEnumerate(t *testing.T) {
	mux, st, sent := otpSetup(t)
	signinOn(t, st)

	known := do(t, mux, "POST", "/login/code", url.Values{"email": {"admin@example.com"}}, nil)
	unknown := do(t, mux, "POST", "/login/code", url.Values{"email": {"nobody@example.com"}}, nil)
	if known.Code != unknown.Code {
		t.Fatalf("known answered %d, unknown %d", known.Code, unknown.Code)
	}
	if bodyString(known) != bodyString(unknown) {
		t.Fatalf("the two answers differ, which names the addresses that exist")
	}
	var cookies int
	for _, rec := range []*http.Response{known.Result(), unknown.Result()} {
		for _, c := range rec.Cookies() {
			if c.Name == signinCookie && c.Value != "" {
				cookies++
			}
		}
	}
	if cookies != 2 {
		t.Fatalf("a cookie was set %d times out of 2: its absence names the unknown address", cookies)
	}
	if len(*sent) != 1 {
		t.Fatalf("want exactly one mail (the known address), got %d", len(*sent))
	}
}

// THE point of the request id: a code read out over the telephone opens
// nothing on the caller's machine.
func TestEmailSigninCodeIsBoundToItsBrowser(t *testing.T) {
	mux, st, sent := otpSetup(t)
	signinOn(t, st)
	askCode(t, mux, "admin@example.com")
	code := digitsOf(t, (*sent)[0].Text)

	// Another browser: no cookie at all, then a cookie of its own.
	if none := do(t, mux, "POST", "/login/code/verify", url.Values{"code": {code}}, nil); none.Code != http.StatusSeeOther ||
		none.Header().Get("Location") != "/login/code" {
		t.Fatalf("a code with no cookie answered %d %q, want a redirect to /login/code",
			none.Code, none.Header().Get("Location"))
	}
	other := askCode(t, mux, "someone@example.com")
	if rec := do(t, mux, "POST", "/login/code/verify", url.Values{"code": {code}}, other); rec.Code == http.StatusSeeOther {
		t.Fatalf("a code minted for one browser signed in from another")
	}
}

// Single use: the code dies when it is spent.
func TestEmailSigninCodeIsSingleUse(t *testing.T) {
	mux, st, sent := otpSetup(t)
	signinOn(t, st)
	cookie := askCode(t, mux, "admin@example.com")
	code := digitsOf(t, (*sent)[0].Text)

	if first := do(t, mux, "POST", "/login/code/verify", url.Values{"code": {code}}, cookie); first.Code != http.StatusSeeOther {
		t.Fatalf("first use answered %d", first.Code)
	}
	if again := do(t, mux, "POST", "/login/code/verify", url.Values{"code": {code}}, cookie); again.Code == http.StatusSeeOther {
		t.Fatalf("a burned code signed in a second time")
	}
}

// It replaces the PASSWORD, never the second factor: an enrolled account lands
// on the challenge, not on the destination.
func TestEmailSigninStillOwesTheSecondFactor(t *testing.T) {
	mux, st, sent := otpSetup(t)
	signinOn(t, st)
	enrol(t, st)

	cookie := askCode(t, mux, "admin@example.com")
	code := digitsOf(t, (*sent)[0].Text)
	rec := do(t, mux, "POST", "/login/code/verify", url.Values{"code": {code}, "next": {"/app"}}, cookie)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/totp" {
		t.Fatalf("a mailed code skipped the second factor: %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

// The console is never openable by a mailbox: the flow is not mounted on the
// control plane, whatever the setting says.
func TestEmailSigninIsNotOnTheControlPlane(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	signinOn(t, st)
	if err := st.SetSetting(context.Background(), store.SettingSMTP,
		mail.Config{Host: "smtp.example.com", Port: 587, From: "gw@example.com"}); err != nil {
		t.Fatal(err)
	}
	h := NewAdmin(st, session.NewManager(st))
	h.Mailer = func(context.Context, mail.Message) error { return nil }
	mux := http.NewServeMux()
	h.Register(mux)

	for _, c := range []struct {
		method, path string
	}{{"GET", "/login/code"}, {"POST", "/login/code"}, {"POST", "/login/code/verify"}} {
		if rec := do(t, mux, c.method, c.path, url.Values{"email": {"admin@example.com"}}, nil); rec.Code != http.StatusNotFound {
			t.Fatalf("the control plane answered %s %s with %d, want 404", c.method, c.path, rec.Code)
		}
	}
}
