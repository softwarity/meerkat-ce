package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
)

// getLogout sends GET /logout with the session cookie and the browser's
// Sec-Fetch-Site stamp ("" sends none).
func getLogout(t *testing.T, mux http.Handler, cookie *http.Cookie, site string, extra ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/logout", nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if site != "" {
		req.Header.Set("Sec-Fetch-Site", site)
	}
	for i := 0; i+1 < len(extra); i += 2 {
		req.Header.Set(extra[i], extra[i+1])
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func signedIn(t *testing.T, mux *http.ServeMux) *http.Cookie {
	t.Helper()
	rec := postLogin(t, mux, url.Values{"username": {"admin"}, "password": {"s3cret"}})
	if rec.Code != http.StatusSeeOther || len(rec.Result().Cookies()) == 0 {
		t.Fatalf("sign-in: %d", rec.Code)
	}
	return rec.Result().Cookies()[0]
}

func alive(sm *session.Manager, cookie *http.Cookie) bool {
	check := httptest.NewRequest(http.MethodGet, "/", nil)
	check.AddCookie(cookie)
	_, err := sm.Resolve(context.Background(), check)
	return err == nil
}

func signouts(t *testing.T, st *store.Store) int {
	t.Helper()
	evs, err := st.ListAuditEvents(context.Background(), store.AuditFilter{Kind: store.AuditKindSecurity})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range evs {
		if e.Action == secSignout {
			n++
		}
	}
	return n
}

// An application's own "sign out" button, on this host or a neighbouring
// subdomain, sends the browser to /logout: the session ends at once, exactly
// as the POST ends it, and the person lands on the sign-in page.
func TestAnApplicationsSignOutLinkSignsOut(t *testing.T) {
	for _, site := range []string{"same-origin", "same-site"} {
		mux, sm, st := setupAll(t)
		cookie := signedIn(t, mux)
		rec := getLogout(t, mux, cookie, site)
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
			t.Fatalf("%s: %d to %q, want 303 to /login", site, rec.Code, rec.Header().Get("Location"))
		}
		if alive(sm, cookie) {
			t.Fatalf("%s: the session survived the sign-out", site)
		}
		cleared := false
		for _, c := range rec.Result().Cookies() {
			if c.Name == cookie.Name && c.MaxAge < 0 {
				cleared = true
			}
		}
		if !cleared {
			t.Errorf("%s: the cookie was not cleared", site)
		}
		if n := signouts(t, st); n != 1 {
			t.Errorf("%s: %d sign-outs in the security trail, want 1", site, n)
		}
	}
}

// Anything else asks first: another site, an address typed in, a client that
// says nothing, a browser fetching ahead of a click. The page's button is the
// ordinary POST, and nothing has ended yet.
func TestASignOutFromElsewhereAsksFirst(t *testing.T) {
	mux, sm, st := setupAll(t)
	cookie := signedIn(t, mux)
	cases := map[string]*httptest.ResponseRecorder{
		"cross-site": getLogout(t, mux, cookie, "cross-site"),
		"none":       getLogout(t, mux, cookie, "none"),
		"no header":  getLogout(t, mux, cookie, ""),
		"prefetch":   getLogout(t, mux, cookie, "same-origin", "Sec-Purpose", "prefetch"),
	}
	for name, rec := range cases {
		if rec.Code != http.StatusOK {
			t.Errorf("%s: %d, want the confirmation page", name, rec.Code)
			continue
		}
		body := bodyString(rec)
		if !strings.Contains(body, `<form method="post" action="/logout">`) ||
			!strings.Contains(body, messages["en"]["signOutAsk"]) {
			t.Errorf("%s: not the confirmation page:\n%s", name, body)
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s: the confirmation may be cached: %q", name, rec.Header().Get("Cache-Control"))
		}
	}
	if !alive(sm, cookie) {
		t.Fatal("a GET from elsewhere ended the session")
	}
	if n := signouts(t, st); n != 0 {
		t.Fatalf("%d sign-outs recorded, want none", n)
	}

	// The page's button: a POST from our own page, which signs out.
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Fatalf("POST: %d to %q", rec.Code, rec.Header().Get("Location"))
	}
	if alive(sm, cookie) {
		t.Fatal("the confirmation's POST did not sign out")
	}
}

// Nobody signed in: nothing to confirm, the sign-in page.
func TestASignOutWithNoSessionGoesToSignIn(t *testing.T) {
	mux, _ := setup(t)
	for _, site := range []string{"same-origin", "cross-site", ""} {
		rec := getLogout(t, mux, nil, site)
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
			t.Errorf("%q: %d to %q, want 303 to /login", site, rec.Code, rec.Header().Get("Location"))
		}
	}
}

// The POST is what it was: a sign-out from our own pages, and a same-site or
// cross-site POST is still refused by the write guard (guard.go). Only the
// data plane takes the GET: the control plane has no application behind it.
func TestThePostSignOutIsUnchanged(t *testing.T) {
	mux, sm, st := setupAll(t)
	cookie := signedIn(t, mux)
	for _, site := range []string{"same-site", "cross-site"} {
		req := httptest.NewRequest(http.MethodPost, "/logout", nil)
		req.Header.Set("Sec-Fetch-Site", site)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("POST from %s: %d, want 403", site, rec.Code)
		}
	}
	if !alive(sm, cookie) {
		t.Fatal("a refused POST ended the session")
	}
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Fatalf("POST: %d to %q", rec.Code, rec.Header().Get("Location"))
	}
	if alive(sm, cookie) {
		t.Fatal("the session survived the POST")
	}
	if n := signouts(t, st); n != 1 {
		t.Errorf("%d sign-outs in the security trail, want 1", n)
	}

	admin := http.NewServeMux()
	NewAdmin(st, sm).Register(admin)
	if rec := getLogout(t, admin, nil, "same-origin"); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /logout on the control plane: %d, want 405", rec.Code)
	}
}
