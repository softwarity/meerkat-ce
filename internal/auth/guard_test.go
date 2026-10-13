package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// registerAgent registers an agent client the way one does, and returns its id.
func registerAgent(t *testing.T, base string) string {
	t.Helper()
	body := `{"client_name":"Claude Code","redirect_uris":["https://attacker.example/cb"]}`
	res, err := http.Post(base+"/oauth/register", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	var reg struct {
		ClientID string `json:"client_id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&reg); err != nil || reg.ClientID == "" {
		t.Fatalf("registration: %v %+v", err, reg)
	}
	return reg.ClientID
}

// THE ATTACK (P0-03 of the 2026-10-12 audit): an agent registers itself as
// "Claude Code" with a redirect of its own, then a page on the data plane of
// the same host - same-site to the console, so the browser attaches root's
// cookie - submits the consent form. It used to come back as a 303 carrying a
// code, exchanged for a full control-plane token.
func TestTheConsentCannotBeSubmittedFromAnotherPage(t *testing.T) {
	srv, _, cookie := setupAgentFlow(t)
	client := registerAgent(t, srv.URL)
	sum := sha256.Sum256([]byte("a-verifier-long-enough-to-be-one-0123456789"))
	ask := url.Values{
		"response_type": {"code"}, "client_id": {client},
		"redirect_uri":   {"https://attacker.example/cb"},
		"code_challenge": {base64.RawURLEncoding.EncodeToString(sum[:])}, "code_challenge_method": {"S256"},
	}
	form := url.Values{"request": {ask.Encode()}, "approve": {"yes"}, "scope": {store.ScopeFull}}

	for _, c := range []struct {
		name    string
		headers map[string]string
		refused bool
	}{
		{"same-site page (the neighbouring port)", map[string]string{"Sec-Fetch-Site": "same-site"}, true},
		{"cross-site page", map[string]string{"Sec-Fetch-Site": "cross-site"}, true},
		{"an older browser, foreign Origin", map[string]string{"Origin": "http://attacker.example"}, true},
		{"a sandboxed frame", map[string]string{"Origin": "null"}, true},
		{"the console's own page", map[string]string{"Sec-Fetch-Site": "same-origin"}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", srv.URL+"/oauth/authorize", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(cookie)
			for k, v := range c.headers {
				req.Header.Set(k, v)
			}
			res, err := http.DefaultTransport.RoundTrip(req)
			if err != nil {
				t.Fatal(err)
			}
			_ = res.Body.Close()
			loc := res.Header.Get("Location")
			if c.refused {
				if res.StatusCode != http.StatusForbidden || strings.Contains(loc, "code=") {
					t.Fatalf("a forged consent answered %d -> %q", res.StatusCode, loc)
				}
				return
			}
			if res.StatusCode != http.StatusSeeOther || !strings.Contains(loc, "code=") {
				t.Fatalf("the console's own approval answered %d -> %q", res.StatusCode, loc)
			}
		})
	}
}

// The consent says where the code goes - the one thing a look-alike name
// cannot borrow - and asks for no frame at all.
func TestTheConsentNamesWhereTheAnswerGoes(t *testing.T) {
	srv, _, cookie := setupAgentFlow(t)
	client := registerAgent(t, srv.URL)
	ask := url.Values{
		"response_type": {"code"}, "client_id": {client}, "redirect_uri": {"https://attacker.example/cb"},
		"code_challenge": {"x"}, "code_challenge_method": {"S256"},
	}
	req, _ := http.NewRequest("GET", srv.URL+"/oauth/authorize?"+ask.Encode(), nil)
	req.AddCookie(cookie)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if !strings.Contains(string(body), "<strong>attacker.example</strong>") {
		t.Fatalf("the consent does not say where the answer goes:\n%s", body)
	}
	if got := res.Header.Get("Content-Security-Policy"); got != "frame-ancestors 'none'" {
		t.Fatalf("the consent may be framed: %q", got)
	}
	if res.Header.Get("X-Frame-Options") != "DENY" {
		t.Fatalf("X-Frame-Options: %q", res.Header.Get("X-Frame-Options"))
	}
}

// The same guard on every write of the sign-in pages and the profile, on both
// planes: turning the second factor off from a neighbouring page is refused,
// and so is a forged sign-in. What a forgery cannot abuse stays open: the
// agent's own calls, and SAML's answer, cross-site by design.
func TestEveryAuthWriteRefusesAnotherSitesPage(t *testing.T) {
	mux, _, _, _ := registerSetup(t)
	for _, path := range []string{"/login", "/profile/mfa", "/profile/email", "/profile/passkeys/delete", "/profile/sessions", "/logout", "/forgot-password"} {
		rec := send(t, &sameSite{mux}, "POST", "", path, url.Values{"step": {"disable"}})
		if rec.Code != http.StatusForbidden {
			t.Errorf("POST %s from a same-site page: %d", path, rec.Code)
		}
	}
	// A program with no browser signals is not a forgery: it passes the guard
	// (and meets the handler's own rules).
	if rec := send(t, mux, "POST", "", "/login", url.Values{"username": {"x"}, "password": {"y"}}); rec.Code == http.StatusForbidden {
		t.Errorf("a plain client was refused by the cross-site guard")
	}
	for _, p := range []string{"POST /oauth/register", "POST /oauth/token", "POST /oauth/revoke", "POST /login/{provider}/callback"} {
		if !crossSiteOpen[p] {
			t.Errorf("%s must stay open to a foreign origin", p)
		}
	}
	// Every page says who may frame it.
	if rec := send(t, mux, "GET", "", "/login", nil); rec.Header().Get("Content-Security-Policy") != "frame-ancestors 'self'" {
		t.Errorf("the sign-in page may be framed by anybody: %q", rec.Header().Get("Content-Security-Policy"))
	}
}

// sameSite stamps every request as a browser does for a page of a
// neighbouring port or subdomain.
type sameSite struct{ next http.Handler }

func (s *sameSite) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Header.Set("Sec-Fetch-Site", "same-site")
	s.next.ServeHTTP(w, r)
}

// A session alone no longer changes the address: the current password is
// asked first, where the account has one, and a wrong one changes nothing.
func TestAnAddressChangeAsksForThePassword(t *testing.T) {
	mux, _, st, box := registerSetup(t)
	aliceWithPassword(t, st, "pass-1234-x")
	sess := sessionCookieOf(do(t, mux, "POST", "/login", url.Values{"username": {"alice"}, "password": {"pass-1234-x"}}, nil))
	for _, current := range []string{"", "not-the-password"} {
		rec := send(t, mux, "POST", "", "/profile/email", url.Values{"email": {"thief@evil.example"}, "current": {current}}, sess)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("address change with password %q: %d", current, rec.Code)
		}
	}
	if len(box.forRecipient("thief@evil.example")) != 0 {
		t.Fatalf("a confirmation went to the new address without the password")
	}
}
