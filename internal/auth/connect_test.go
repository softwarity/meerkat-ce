package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/idp"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
)

// Connecting an authority to the account one is ALREADY signed in as. This is
// what makes "self-registration: refused" usable: without it, an authority can
// only ever reach someone whose address an administrator happened to type
// correctly, and nobody on either side can do anything about a mismatch.
func TestConnectAttachesToTheSignedInAccount(t *testing.T) {
	mux, st := setupWithStore(t)
	sm := session.NewManager(st)
	ctx := context.Background()
	if err := st.SaveAuthProvider(ctx, store.AuthProvider{
		ID: "acme", Kind: store.ProviderGitHub, Name: "Acme SSO", Enabled: true,
		AutoCreate: store.PolicyNo,
		Config:     map[string]any{"clientId": "meerkat", "clientSecret": "s3cret"},
	}); err != nil {
		t.Fatal(err)
	}
	sc := sessionCookieOf(postLogin(t, mux, url.Values{
		"username": {"admin"}, "password": {"s3cret"}}))
	if sc == nil {
		t.Fatal("no session cookie")
	}

	// Security leads to the page; the page is where they are managed.
	if sec := bodyString(do(t, mux, "GET", "/profile/security", nil, sc)); !strings.Contains(sec, "/profile/authorities") {
		t.Fatalf("the security page does not lead to the authorities:\n%.600s", sec)
	}
	page := bodyString(do(t, mux, "GET", "/profile/authorities", nil, sc))
	if !strings.Contains(page, "/profile/connect/acme") {
		t.Fatalf("the authorities page offers no way to connect one:\n%.600s", page)
	}

	// The link is the ordinary redirect, started in LINK mode.
	start := do(t, mux, "GET", "/profile/connect/acme", nil, sc)
	if start.Code != http.StatusSeeOther {
		t.Fatalf("connect start: %d", start.Code)
	}
	if loc := start.Header().Get("Location"); !strings.Contains(loc, "github.com/login/oauth") {
		t.Fatalf("connect did not leave for the authority: %q", loc)
	}

	// What the authority eventually says, attached to the session's account.
	h := New(st, sm)
	req := httptest.NewRequest("GET", "/profile/security", nil)
	req.AddCookie(sc)
	h.completeConnect(httptest.NewRecorder(), req,
		store.AuthProvider{ID: "acme", Name: "Acme SSO"},
		idp.Identity{Subject: "octocat", Email: "someone-else@example.org"})

	u, err := st.UserByIdentity(ctx, "acme", "octocat")
	if err != nil {
		t.Fatalf("the identity was not attached: %v", err)
	}
	if u.Username != "admin" {
		t.Fatalf("attached to %q, want the signed-in account", u.Username)
	}
	// The address did NOT have to match, and was not touched: that is the
	// whole point of connecting from the profile rather than guessing.
	if u.Email == "someone-else@example.org" {
		t.Fatal("connecting an authority overwrote the account's address")
	}

	// Now the page says so, and offers the way back out.
	page = bodyString(do(t, mux, "GET", "/profile/authorities", nil, sc))
	if !strings.Contains(page, "/profile/disconnect/acme") {
		t.Fatalf("a connected authority cannot be detached:\n%.600s", page)
	}
	if code := do(t, mux, "POST", "/profile/disconnect/acme", nil, sc).Code; code != http.StatusSeeOther {
		t.Fatalf("disconnect: %d", code)
	}
	if _, err := st.UserByIdentity(ctx, "acme", "octocat"); err == nil {
		t.Fatal("the identity survived a disconnect")
	}
}

// An authority account identifies ONE person here, or the next sign-in through
// it would be a coin toss.
func TestConnectRefusesSomebodyElsesIdentity(t *testing.T) {
	mux, st := setupWithStore(t)
	sm := session.NewManager(st)
	ctx := context.Background()
	if err := st.SaveAuthProvider(ctx, store.AuthProvider{
		ID: "acme", Kind: store.ProviderGitHub, Name: "Acme SSO", Enabled: true,
		Config: map[string]any{"clientId": "meerkat", "clientSecret": "s3cret"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser(ctx, store.User{
		ID: "u2", Username: "other", PasswordHash: "x", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := st.LinkIdentity(ctx, "acme", "octocat", "u2", nil); err != nil {
		t.Fatal(err)
	}
	sc := sessionCookieOf(postLogin(t, mux, url.Values{
		"username": {"admin"}, "password": {"s3cret"}}))

	h := New(st, sm)
	req := httptest.NewRequest("GET", "/profile/security", nil)
	req.AddCookie(sc)
	rec := httptest.NewRecorder()
	h.completeConnect(rec, req, store.AuthProvider{ID: "acme", Name: "Acme SSO"},
		idp.Identity{Subject: "octocat"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("stealing an identity answered %d, want 409", rec.Code)
	}
	if u, err := st.UserByIdentity(ctx, "acme", "octocat"); err != nil || u.ID != "u2" {
		t.Fatal("the identity moved to another account")
	}
}

// Disconnecting the LAST way in would lock someone out with their own click,
// and the page offering it sits behind the login they just closed.
func TestDisconnectRefusesTheLastWayIn(t *testing.T) {
	mux, st := setupWithStore(t)
	ctx := context.Background()
	if err := st.SaveAuthProvider(ctx, store.AuthProvider{
		ID: "acme", Kind: store.ProviderGitHub, Name: "Acme SSO", Enabled: true,
		Config: map[string]any{"clientId": "meerkat", "clientSecret": "s3cret"},
	}); err != nil {
		t.Fatal(err)
	}
	// An account born at an authority: no local password, one identity.
	if err := st.CreateUser(ctx, store.User{
		ID: "u3", Username: "external", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := st.LinkIdentity(ctx, "acme", "sub-3", "u3", nil); err != nil {
		t.Fatal(err)
	}
	sm := session.NewManager(st)
	rec := httptest.NewRecorder()
	if _, err := sm.Issue(ctx, rec, httptest.NewRequest("POST", "/login", nil), "u3"); err != nil {
		t.Fatal(err)
	}
	var sc *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == session.CookieName {
			sc = c
		}
	}

	if code := do(t, mux, "POST", "/profile/disconnect/acme", nil, sc).Code; code != http.StatusUnprocessableEntity {
		t.Fatalf("closing the only way in answered %d, want 422", code)
	}
	if _, err := st.UserByIdentity(ctx, "acme", "sub-3"); err != nil {
		t.Fatal("the only way in was removed anyway")
	}
	// The page says so rather than offering a button that refuses.
	page := bodyString(do(t, mux, "GET", "/profile/authorities", nil, sc))
	if strings.Contains(page, "/profile/disconnect/acme") {
		t.Fatal("the page offered to close the only way in")
	}
}
