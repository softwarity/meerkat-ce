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

// The language menu writes the choice on the ACCOUNT, not only in a cookie
// (I18N-04). A cookie is a browser's memory; this is the person's - it is what
// greets them in their language on another machine, and what finally sends
// their transactional mail in the language they picked rather than the one an
// administrator typed.
func TestPickedLocaleLandsOnTheAccount(t *testing.T) {
	mux, sm, st := setupFlow(t)
	ctx := context.Background()
	if err := st.SetSetting(ctx, store.SettingLanguages, []string{"en", "fr"}); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	if _, err := sm.Issue(ctx, rec, httptest.NewRequest("POST", "/login", nil), "u1"); err != nil {
		t.Fatal(err)
	}
	cookies := rec.Result().Cookies()

	post := func(body string, withSession bool) int {
		req := httptest.NewRequest(http.MethodPost, "/meerkat/locale", strings.NewReader(body))
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

	if code := post(`{"locale":"fr"}`, true); code != http.StatusNoContent {
		t.Fatalf("picking an offered language answered %d", code)
	}
	u, err := st.GetUserByID(ctx, "u1")
	if err != nil || u.Locale != "fr" {
		t.Fatalf("locale not stored: %q (%v)", u.Locale, err)
	}

	// A code the application does not offer is refused rather than stored: the
	// field feeds a catalogue lookup and an upstream header.
	if code := post(`{"locale":"kl"}`, true); code != http.StatusUnprocessableEntity {
		t.Errorf("an unoffered language answered %d, want 422", code)
	}
	u, _ = st.GetUserByID(ctx, "u1")
	if u.Locale != "fr" {
		t.Errorf("a refused language overwrote the stored one: %q", u.Locale)
	}

	// Signed out - which is most of the flow pages - there is no account to
	// write to, and the cookie has already done the work.
	if code := post(`{"locale":"en"}`, false); code != http.StatusNoContent {
		t.Errorf("an anonymous caller answered %d, want 204", code)
	}
}

// The language follows the person to a browser that never knew them: signing
// in is when we learn who is asking, and the cookie every page reads is set
// from what they chose, wherever they chose it.
func TestSignInCarriesTheChosenLanguage(t *testing.T) {
	ctx := context.Background()
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	hash, _ := bcrypt.GenerateFromPassword([]byte("s3cret"), bcrypt.MinCost)
	if err := st.CreateUser(ctx, store.User{
		ID: "u1", Username: "jane", PasswordHash: string(hash), Enabled: true, Locale: "vi",
	}); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	New(st, session.NewManager(st)).Register(mux)

	form := url.Values{"username": {"jane"}, "password": {"s3cret"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// This browser says French; the person chose Vietnamese, elsewhere.
	req.Header.Set("Accept-Language", "fr")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("sign-in answered %d", rec.Code)
	}
	var got string
	for _, c := range rec.Result().Cookies() {
		if c.Name == "MEERKAT_LANG" {
			got = c.Value
		}
	}
	if got != "vi" {
		t.Errorf("the language cookie is %q, want the account's own", got)
	}
}
