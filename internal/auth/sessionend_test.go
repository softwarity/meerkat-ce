package auth

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
	"golang.org/x/crypto/bcrypt"
)

// A session that ends sends every open page to the login form at once. Asking
// someone to sign in five times, once per tab, would be the timeout's real
// cost - so the tab where they did it says so, and the others leave on their
// own.
func TestLoginPageFollowsASessionOpenedElsewhere(t *testing.T) {
	mux, _ := setup(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/login?next=/app", nil))
	body, _ := io.ReadAll(rec.Result().Body)
	page := string(body)

	for _, want := range []string{"meerkat-session", session.UntilCookieName, "'alive'"} {
		if !strings.Contains(page, want) {
			t.Errorf("the login page cannot hear a session opened elsewhere: no %s", want)
		}
	}
	// Same-site only: the destination comes from a query parameter, and a page
	// that jumps to whatever it says would be an open redirect wearing a
	// broadcast for a trigger.
	if !strings.Contains(page, "startsWith('/')") || !strings.Contains(page, "!back.startsWith('//')") {
		t.Errorf("the return path is not checked:\n%s", page)
	}
}

// The two ports carry different sessions, so the login page of one must read
// the other's deadline no more than it reads its cookie.
func TestAdminLoginPageWatchesTheAdminDeadline(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	hash, _ := bcrypt.GenerateFromPassword([]byte("s3cret"), bcrypt.MinCost)
	if err := st.CreateUser(context.Background(), store.User{
		ID: "u1", Username: "admin", PasswordHash: string(hash), Root: true, Enabled: true}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	mux := http.NewServeMux()
	NewAdmin(st, session.NewManager(st, session.ForAdminPlane())).Register(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/login", nil))
	body, _ := io.ReadAll(rec.Result().Body)
	page := string(body)
	if !strings.Contains(page, session.AdminUntilCookieName) {
		t.Errorf("the console's login page watches the wrong plane's deadline")
	}
	if strings.Contains(page, session.UntilCookieName+"=") {
		t.Errorf("the console's login page reads the data plane's deadline")
	}
}
