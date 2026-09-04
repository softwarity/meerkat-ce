package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// The direction has to reach the PAGE, not just the helper: one attribute on
// <html> is what mirrors the whole layout, and it is the kind of thing that
// silently stops being rendered when a template is reshuffled.
func TestSignInPageCarriesTheWritingDirection(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.SetSetting(context.Background(), store.SettingLanguages, []string{"ar", "en"}); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	New(st, session.NewManager(st)).Register(mux)

	page := func(lang string) string {
		req := httptest.NewRequest(http.MethodGet, "/login", nil)
		req.AddCookie(&http.Cookie{Name: "MEERKAT_LANG", Value: lang})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec.Body.String()
	}

	if got := page("ar"); !strings.Contains(got, `<html lang="ar" dir="rtl">`) {
		t.Errorf("Arabic page is not marked right to left: %q", firstLine(got))
	}
	if got := page("en"); !strings.Contains(got, `<html lang="en" dir="ltr">`) {
		t.Errorf("English page lost its direction: %q", firstLine(got))
	}
	// And the words themselves arrived.
	if got := page("ar"); !strings.Contains(got, messages["ar"]["signIn"]) {
		t.Error("the Arabic catalogue did not reach the page")
	}
}
