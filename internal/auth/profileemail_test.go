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
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// An account can perfectly well have no address - seeded, made by an
// administrator in a hurry, or handed over by a directory that kept it - and
// the address is what lets someone back in when they lose their password.
// Setting it is theirs to do, not an administrator's to be asked for.
func TestProfileEmailIsSelfService(t *testing.T) {
	ctx := context.Background()
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	for _, u := range []store.User{
		{ID: "u1", Username: "jane", Enabled: true, PasswordHash: "x"},
		{ID: "u2", Username: "bob", Enabled: true, PasswordHash: "x", Email: "taken@example.test"},
	} {
		if err := st.CreateUser(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	sm := session.NewManager(st)
	mux := http.NewServeMux()
	New(st, sm).Register(mux)

	rec := httptest.NewRecorder()
	if _, err := sm.Issue(ctx, rec, httptest.NewRequest("POST", "/login", nil), "u1"); err != nil {
		t.Fatal(err)
	}
	cookies := rec.Result().Cookies()
	post := func(email string) int {
		req := httptest.NewRequest(http.MethodPost, "/profile/email",
			strings.NewReader(url.Values{"email": {email}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for _, c := range cookies {
			req.AddCookie(c)
		}
		out := httptest.NewRecorder()
		mux.ServeHTTP(out, req)
		return out.Code
	}

	if code := post("jane@example.test"); code != http.StatusSeeOther {
		t.Fatalf("setting an address answered %d", code)
	}
	u, _ := st.GetUserByID(ctx, "u1")
	if u.Email != "jane@example.test" {
		t.Fatalf("address not stored: %q", u.Email)
	}

	// Not an address: refused, and the stored one stands.
	if code := post("jane(at)example"); code != http.StatusUnprocessableEntity {
		t.Errorf("a malformed address answered %d, want 422", code)
	}
	// Someone else's: refused too - "reset the password of that address" has
	// to name one account.
	if code := post("taken@example.test"); code != http.StatusConflict {
		t.Errorf("an address another account holds answered %d, want 409", code)
	}
	u, _ = st.GetUserByID(ctx, "u1")
	if u.Email != "jane@example.test" {
		t.Errorf("a refused address overwrote the stored one: %q", u.Email)
	}
}

// The console is a plane too, and whoever administers the gateway needs the
// same things: a photo, a passkey, an address. The pages were data-plane only.
func TestProfileServedOnBothPlanes(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.CreateUser(ctx, store.User{ID: "u1", Username: "root", Enabled: true, PasswordHash: "x", Root: true}); err != nil {
		t.Fatal(err)
	}
	sm := session.NewManager(st, session.ForAdminPlane())
	mux := http.NewServeMux()
	NewAdmin(st, sm).Register(mux)

	rec := httptest.NewRecorder()
	if _, err := sm.Issue(ctx, rec, httptest.NewRequest("POST", "/login", nil), "u1"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/profile", "/profile/security", "/profile/history"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		for _, c := range rec.Result().Cookies() {
			req.AddCookie(c)
		}
		out := httptest.NewRecorder()
		mux.ServeHTTP(out, req)
		if out.Code != http.StatusOK {
			t.Errorf("the console answered %d on %s", out.Code, path)
		}
	}

	// What only means something in front of an application stays there: the
	// personal API tokens page, and the apps menu.
	req := httptest.NewRequest(http.MethodGet, "/profile/tokens", nil)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	out := httptest.NewRecorder()
	mux.ServeHTTP(out, req)
	if out.Code != http.StatusNotFound {
		t.Errorf("the data plane's token page answered %d on the console", out.Code)
	}
}

// The console's profile needs a way back, for the same reason the data plane's
// carries its apps menu: someone who came to add a passkey has finished, and
// the browser's back button is not an answer a product gives.
func TestConsoleProfileLeadsBack(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.CreateUser(ctx, store.User{ID: "u1", Username: "root", Enabled: true, PasswordHash: "x", Root: true, Dev: true}); err != nil {
		t.Fatal(err)
	}
	sm := session.NewManager(st, session.ForAdminPlane())
	mux := http.NewServeMux()
	NewAdmin(st, sm).Register(mux)
	rec := httptest.NewRecorder()
	if _, err := sm.Issue(ctx, rec, httptest.NewRequest("POST", "/login", nil), "u1"); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	out := httptest.NewRecorder()
	mux.ServeHTTP(out, req)
	body := out.Body.String()
	if !strings.Contains(body, `href="/"`) {
		t.Errorf("no way back to the console:\n%.600s", body)
	}
	// And no entry leading to a page this plane does not serve: the developer
	// tooling is about the applications - a certificate their services
	// authenticate, the UI test mode - and its pages live over there.
	if strings.Contains(body, `href="/profile/dev"`) {
		t.Errorf("the console offered the developer page it does not serve:\n%.600s", body)
	}
	// Nor the timezone: it is what an APPLICATION renders this person's dates
	// in, and the console renders its own the browser's way.
	if strings.Contains(body, `action="/profile/timezone"`) {
		t.Errorf("the console offered a timezone it does not use:\n%.600s", body)
	}
}

// The data plane keeps both, being where they mean something.
func TestDataPlaneProfileKeepsTheTimezone(t *testing.T) {
	mux, sm, _ := setupFlow(t)
	ctx := context.Background()
	rec := httptest.NewRecorder()
	if _, err := sm.Issue(ctx, rec, httptest.NewRequest("POST", "/login", nil), "u1"); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	out := httptest.NewRecorder()
	mux.ServeHTTP(out, req)
	if !strings.Contains(out.Body.String(), `action="/profile/timezone"`) {
		t.Error("the application's profile must keep the timezone")
	}
}
