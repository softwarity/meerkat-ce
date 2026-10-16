package auth

import (
	"html"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// The page an application's refused sign-in lands on is a flow page in the
// visitor's language, with the status the provider owes the request, and one
// sentence per reason - an unknown reason the generic one, never a key.
func TestTheAppRefusedPageIsADressedPage(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	speaks(t, st, "en", "fr")
	h := New(st, session.NewManager(st))

	for _, c := range []struct {
		why    string
		status int
		key    string
	}{
		{AppRefusedUnknown, http.StatusBadRequest, "appRefusedUnknown"},
		{AppRefusedReturn, http.StatusBadRequest, "appRefusedReturn"},
		{AppRefusedUnavailable, http.StatusServiceUnavailable, "appRefusedUnavailable"},
		{AppRefusedUnreadable, http.StatusBadRequest, "appRefusedUnreadable"},
		{"from-a-newer-build", http.StatusBadRequest, "appRefusedUnreadable"},
	} {
		req := httptest.NewRequest(http.MethodGet, "/meerkat/oidc/authorize", nil)
		req.Header.Set("Accept-Language", "fr")
		rec := httptest.NewRecorder()
		h.ServeAppRefused(rec, req, c.why)
		body := rec.Body.String()
		if rec.Code != c.status {
			t.Errorf("%s: status %d, want %d", c.why, rec.Code, c.status)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Errorf("%s: content type %q, want a page", c.why, ct)
		}
		for what, want := range map[string]string{
			"the visitor's language": `lang="fr"`,
			"the reason, translated": html.EscapeString(messages["fr"][c.key]),
			"the lead":               html.EscapeString(messages["fr"]["appRefusedLead"]),
			"the language switcher":  "lang-menu",
			"the colour scheme":      "scheme-cycle",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: %s is not on the page", c.why, what)
			}
		}
	}
}
