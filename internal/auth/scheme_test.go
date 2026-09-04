package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// The scheme switch writes the choice on the ACCOUNT, exactly as the language
// menu does (THEME-05). The two sit side by side in the same bar, and one
// following the person to another machine while the other stayed behind is the
// kind of difference nobody can explain to the person it happens to.
func TestPickedSchemeLandsOnTheAccount(t *testing.T) {
	mux, sm, st := setupFlow(t)
	ctx := context.Background()

	rec := httptest.NewRecorder()
	if _, err := sm.Issue(ctx, rec, httptest.NewRequest("POST", "/login", nil), "u1"); err != nil {
		t.Fatal(err)
	}
	cookies := rec.Result().Cookies()

	post := func(body string, withSession bool) int {
		req := httptest.NewRequest(http.MethodPost, "/meerkat/scheme", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if withSession {
			for _, c := range cookies {
				req.AddCookie(c)
			}
		}
		out := httptest.NewRecorder()
		mux.ServeHTTP(out, req)
		return out.Code
	}

	if code := post(`{"scheme":"dark"}`, true); code != http.StatusNoContent {
		t.Fatalf("picking dark answered %d, want 204", code)
	}
	u, err := st.GetUserByID(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme != "dark" {
		t.Fatalf("the account holds %q, want dark", u.Scheme)
	}

	// "auto" is what the pages call following the system, and it is what the
	// account calls nothing: storing the word would make "chose to follow the
	// system" and "never chose" two states nobody can tell apart.
	if code := post(`{"scheme":"auto"}`, true); code != http.StatusNoContent {
		t.Fatalf("going back to auto answered %d, want 204", code)
	}
	if u, _ = st.GetUserByID(ctx, "u1"); u.Scheme != "" {
		t.Errorf("auto was stored as %q, want it cleared", u.Scheme)
	}

	// A value no page offers is refused, and the store names what it allows.
	if code := post(`{"scheme":"sepia"}`, true); code != http.StatusUnprocessableEntity {
		t.Errorf("an invented scheme answered %d, want 422", code)
	}

	// Signed out - which most of the flow pages are - there is no account to
	// write to, and the cookie has already done the work.
	if code := post(`{"scheme":"dark"}`, false); code != http.StatusNoContent {
		t.Errorf("an anonymous caller answered %d, want 204", code)
	}
}

// The scheme follows the person to a browser that never knew them, the way the
// language does: signing in is when we learn who is asking.
func TestSignInCarriesTheChosenScheme(t *testing.T) {
	ctx := context.Background()
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	hash, _ := bcrypt.GenerateFromPassword([]byte("s3cret"), bcrypt.MinCost)
	if err := st.CreateUser(ctx, store.User{
		ID: "u1", Username: "jane", PasswordHash: string(hash), Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetUserScheme(ctx, "u1", "dark"); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	New(st, session.NewManager(st)).Register(mux)

	signIn := func() []*http.Cookie {
		form := url.Values{"username": {"jane"}, "password": {"s3cret"}}
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("sign-in answered %d", rec.Code)
		}
		return rec.Result().Cookies()
	}

	var got string
	for _, c := range signIn() {
		if c.Name == "MEERKAT_SCHEME" {
			got = c.Value
		}
	}
	if got != "dark" {
		t.Fatalf("the scheme cookie is %q, want the account's own", got)
	}

	// An account that chose NOTHING says nothing: this machine's cookie is
	// left alone rather than cleared, because "follow the system" is not a
	// decision to un-choose what someone set here.
	if err := st.SetUserScheme(ctx, "u1", ""); err != nil {
		t.Fatal(err)
	}
	for _, c := range signIn() {
		if c.Name == "MEERKAT_SCHEME" {
			t.Errorf("an account with no scheme still wrote the cookie (%q)", c.Value)
		}
	}
}
