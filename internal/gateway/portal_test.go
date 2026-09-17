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

// routerWithPortal builds a router whose store carries a portal config, so the
// Reload bakes the portal decision into every compiled UI route - the same path
// a live gateway takes when an operator turns the portal on.
func routerWithPortal(t *testing.T, portal store.PortalConfig, routes ...store.Route) *Router {
	t.Helper()
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	for _, r := range routes {
		if err := st.SaveRoute(ctx, r); err != nil {
			t.Fatalf("SaveRoute: %v", err)
		}
	}
	if err := st.SetSetting(ctx, store.SettingPortal, portal); err != nil {
		t.Fatalf("SetSetting portal: %v", err)
	}
	rt := New(st, session.NewManager(st))
	if err := rt.Reload(ctx); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	return rt
}

// TestPortalBarReplacesTheUserButton: on a UI route with the user button on,
// turning the portal on swaps the standalone button for the portal bar (which
// mounts the button itself). The page agent rides either way.
func TestPortalBarReplacesTheUserButton(t *testing.T) {
	upstream := htmlUpstream(t, "/page")
	uiRoute := func() store.Route {
		r := pathRoute("r1", "demo", 1, "/demo/**", upstream.URL,
			routing.Spec{Type: "strip-prefix", Args: map[string]any{"parts": 1}})
		r.IsUI = true
		r.UI = &store.RouteUI{UserButton: store.UserButton{Enabled: true}}
		return r
	}

	// Portal OFF: the standalone user button is injected, no bar.
	off := newRouter(t, uiRoute())
	if _, body := get(t, off, "/demo/page"); !strings.Contains(body, "<meerkat-user-button") ||
		strings.Contains(body, "meerkat-portal-nav") {
		t.Fatalf("portal off: expected the standalone button and no bar:\n%s", body)
	}

	// Portal ON: the bar is injected, the standalone button gives way to it.
	on := routerWithPortal(t,
		store.PortalConfig{Enabled: true, Layout: store.PortalHeader, Side: "left",
			Parents: []store.ModuleParent{{RouteID: "r1"}}},
		uiRoute())
	_, body := get(t, on, "/demo/page")
	if !strings.Contains(body, "<meerkat-portal-nav>") {
		t.Errorf("portal on: the bar should be injected:\n%s", body)
	}
	if !strings.Contains(body, "/meerkat/portal.js") {
		t.Errorf("portal on: portal.js should be injected:\n%s", body)
	}
	if strings.Contains(body, "<meerkat-user-button") {
		t.Errorf("portal on: the standalone button element must give way to the bar:\n%s", body)
	}
	if !strings.Contains(body, "/meerkat/page.js") {
		t.Errorf("portal on: the page agent rides either way:\n%s", body)
	}
}

// TestUnavailableUIPageKeepsThePortalBar: when a UI route's upstream is dead and
// a portal is on, the 502 is an HTML page that still carries the bar, so the
// visitor can navigate away instead of being stranded. With the portal off it
// stays the plain-text 502.
func TestUnavailableUIPageKeepsThePortalBar(t *testing.T) {
	// A server we close at once: connections to it are refused, so the proxy's
	// ErrorHandler fires.
	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	deadURL := dead.URL
	dead.Close()

	uiRoute := func() store.Route {
		r := pathRoute("r1", "demo", 1, "/demo/**", deadURL,
			routing.Spec{Type: "strip-prefix", Args: map[string]any{"parts": 1}})
		r.IsUI = true
		return r
	}

	on := routerWithPortal(t,
		store.PortalConfig{Enabled: true, Layout: store.PortalHeader, Side: "left",
			Parents: []store.ModuleParent{{RouteID: "r1"}}},
		uiRoute())
	res, body := get(t, on, "/demo/x")
	if res.StatusCode != http.StatusBadGateway {
		t.Fatalf("want 502, got %d", res.StatusCode)
	}
	if !strings.Contains(res.Header.Get("Content-Type"), "text/html") {
		t.Errorf("the unavailable page should be HTML, got %q", res.Header.Get("Content-Type"))
	}
	if !strings.Contains(body, "<meerkat-portal-nav>") || !strings.Contains(body, "/meerkat/portal.js") {
		t.Errorf("the unavailable page must carry the portal bar:\n%s", body)
	}

	// Portal off: the plain 502 stands (no bar, not HTML chrome).
	off := newRouter(t, uiRoute())
	res2, body2 := get(t, off, "/demo/x")
	if res2.StatusCode != http.StatusBadGateway {
		t.Fatalf("want 502, got %d", res2.StatusCode)
	}
	if strings.Contains(body2, "meerkat-portal-nav") {
		t.Errorf("portal off: the 502 should stay plain, got:\n%s", body2)
	}
}

// TestPortalFragmentOnlyForUIRoutes: a service route wears no portal bar - the
// bar is a UI-only injection, like the button it replaces.
func TestPortalFragmentOnlyForUIRoutes(t *testing.T) {
	if f := portalFragment(store.Route{IsUI: true}); !strings.Contains(f, "<meerkat-portal-nav>") ||
		!strings.Contains(f, "/meerkat/portal.js") || !strings.Contains(f, "/meerkat/user-button.js") {
		t.Errorf("a UI route should get the full portal fragment, got %q", f)
	}
	if f := portalFragment(store.Route{IsUI: false}); f != "" {
		t.Errorf("a non-UI route should get no portal fragment, got %q", f)
	}
}
