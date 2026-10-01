package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// ACCEPT-LANGUAGE IS THE FLOOR, on every proxied route, declared languages or
// not.
//
// It is not one of the mechanisms an integrator picks - it is what a service
// gets for free, rewritten with the person's own language in front so a backend
// answers in the language they chose rather than the one their browser asked
// for first. An API route declares no language because it serves no page, and
// it still serves somebody who chose one.
//
// This exists because the pivot to Speaks quietly broke it: the filter that
// promotes the language was fitted only to routes with a non-empty list, which
// under the old gateway-wide pool meant every route, and under the new one
// means only the UI routes that were given one.
func TestAcceptLanguageIsPromotedWithoutDeclaringAnything(t *testing.T) {
	var seen string
	up := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Accept-Language")
	}))
	t.Cleanup(up.Close)

	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	// A SERVICE route: no UI, nothing declared, the ordinary case.
	if err := st.SaveRoute(ctx, store.Route{
		ID: "api", Name: "api", Order: 1, Enabled: true,
		Upstream:   up.URL,
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/api/**"}}}},
	}); err != nil {
		t.Fatal(err)
	}
	rt := New(st, session.NewManager(st))
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)

	ask := func(cookie, accept string) string {
		t.Helper()
		seen = ""
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/orders", nil)
		if accept != "" {
			req.Header.Set("Accept-Language", accept)
		}
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: langCookie, Value: cookie})
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		return seen
	}

	// The person's stored choice comes first, whatever the browser prefers.
	if got := ask("fr", "de, en;q=0.8"); got != "fr, de, en;q=0.8" {
		t.Errorf("with a chosen language the upstream reads %q", got)
	}
	// No choice: the browser's own order, untouched.
	if got := ask("", "de, en;q=0.8"); got != "de, en;q=0.8" {
		t.Errorf("with no choice the upstream reads %q", got)
	}
	// Nobody said anything: nothing invented, and above all not an empty header.
	if got := ask("", ""); got != "" {
		t.Errorf("with nothing asked the upstream reads %q", got)
	}
}

// THE MENU ON A PAGE OFFERS WHAT THAT PAGE IS WRITTEN IN, whole - including a
// language Meerkat cannot render for itself.
//
// The two lists are deliberately different. The sign-in page offers the union
// of what the routes speak INTERSECTED with what Meerkat embeds, because it is
// Meerkat's own page and it can only be drawn in a language it has. The account
// menu on an application's page is that application's, and an application
// written in a language of its own still offers it there.
func TestTheMenuOnAPageOffersThatPagesLanguages(t *testing.T) {
	codes := []string{"fr", "ja-JP", "lf"}
	r := store.Route{
		ID: "web", IsUI: true,
		UI: &store.RouteUI{UserButton: store.UserButton{Enabled: true}},
	}
	if frag := userButtonFragment(r, codes); !strings.Contains(frag, `languages="fr,ja-JP,lf"`) {
		t.Errorf("the standalone button does not carry the page's own languages:\n%s", frag)
	}
	// And the portal bar, which mounts the same button: portal.json carries the
	// gateway's whole offer for want of knowing which page asked, so the
	// injection names them here.
	if frag := portalFragment(r, codes); !strings.Contains(frag, `languages="fr,ja-JP,lf"`) {
		t.Errorf("the portal bar does not carry the page's own languages:\n%s", frag)
	}
}
