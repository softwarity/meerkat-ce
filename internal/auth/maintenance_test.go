package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// TestUnavailablePageOffersTheWayBack: a closed door is a dead end unless it
// says where the visitor came from. The page is answered by the GATEWAY, not
// proxied from an application, so it carries no portal bar and no link of the
// application's own - without this, the only way out is the browser's own
// button.
//
// The referrer is untrusted, so only its path is used and only when the host is
// this one: the link cannot become a way off this site, whatever the header
// says.
func TestUnavailablePageOffersTheWayBack(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	h := New(st, session.NewManager(st))

	render := func(referer string) string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "http://gw.example/app/broken", nil)
		if referer != "" {
			req.Header.Set("Referer", referer)
		}
		rec := httptest.NewRecorder()
		h.ServeMaintenance(rec, req, store.ReasonIncident, 0, "")
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("want 503, got %d", rec.Code)
		}
		return rec.Body.String()
	}

	// Came from another page of this gateway: the way back is offered, as a
	// path relative to this site.
	body := render("http://gw.example/shop/orders?page=2")
	if !strings.Contains(body, `href="/shop/orders?page=2"`) {
		t.Errorf("the way back was not offered:\n%s", body)
	}

	// From somewhere else, from the very path that failed, or from nowhere:
	// nothing offered, and above all no link off this site.
	for _, ref := range []string{
		"", "https://evil.test/take-me", "http://gw.example/app/broken",
	} {
		if b := render(ref); strings.Contains(b, "evil.test") || strings.Contains(b, `href="/app/broken"`) {
			t.Errorf("referer %q produced a link it should not:\n%s", ref, b)
		}
	}
}
