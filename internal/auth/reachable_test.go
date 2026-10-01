package auth

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

func uiRoute(id string, order int, authenticated bool, requiredRole string) store.Route {
	access := store.Access{}
	if authenticated {
		access.Level = store.AccessAuth
	}
	if requiredRole != "" {
		access.Roles = []string{requiredRole}
	}
	return store.Route{
		ID: id, Name: id, Order: order, Enabled: true, IsUI: true,
		Access:   access,
		Upstream: "http://upstream.test",
		Predicates: []routing.Spec{
			{Type: "path", Args: map[string]any{"patterns": []any{"/" + id + "/**"}}},
		},
	}
}

// catalogue puts the named routes in the portal catalogue, in the order given,
// under the mode asked for. A UI route is no longer offered because it carries
// a label: it is offered because it is in THIS list (PORTAL-03).
func seedCatalogue(t *testing.T, st *store.Store, mode string, ids ...string) {
	t.Helper()
	cfg := store.PortalConfig{Mode: mode}
	for _, id := range ids {
		cfg.Entries = append(cfg.Entries, store.PortalEntry{RouteID: id, Label: id})
	}
	routes, err := st.ListRoutes(context.Background())
	if err != nil {
		t.Fatalf("ListRoutes: %v", err)
	}
	if err := store.SanitizePortalConfig(&cfg, routes); err != nil {
		t.Fatalf("SanitizePortalConfig: %v", err)
	}
	if err := st.SetSetting(context.Background(), store.SettingPortal, cfg); err != nil {
		t.Fatalf("SetSetting portal: %v", err)
	}
}

// TestProfileHubAndUserButtonListReachableApps: the profile hub and the
// user-button JSON offer the UI routes THIS session may open - public and
// authenticated ones, never a role-gated one the user does not hold.
func TestProfileHubAndUserButtonListReachableApps(t *testing.T) {
	mux, _, st := mfaSetup(t)
	ctx := context.Background()
	for _, rt := range []store.Route{
		uiRoute("shop", 1, false, ""),
		uiRoute("intranet", 2, true, ""),
		uiRoute("ops", 3, true, "operations"),
	} {
		if err := st.SaveRoute(ctx, rt); err != nil {
			t.Fatalf("SaveRoute %s: %v", rt.ID, err)
		}
	}
	seedCatalogue(t, st, store.PortalModeLinks, "shop", "intranet", "ops")

	login := do(t, mux, "POST", "/login", url.Values{"username": {"admin"}, "password": {"s3cret"}}, nil)
	sc := sessionCookieOf(login)

	hub := do(t, mux, "GET", "/profile", nil, sc)
	body := bodyString(hub)
	if !strings.Contains(body, `href="/shop"`) || !strings.Contains(body, `href="/intranet"`) {
		t.Fatalf("profile hub must link the reachable apps:\n%.600s", body)
	}
	if strings.Contains(body, `href="/ops"`) {
		t.Fatalf("profile hub must not link a role-gated app the user does not hold")
	}

	btn := do(t, mux, "GET", "/meerkat/user-button.json", nil, sc)
	payload := bodyString(btn)
	if !strings.Contains(payload, `"/shop"`) || !strings.Contains(payload, `"/intranet"`) {
		t.Fatalf("user-button apps missing: %s", payload)
	}
	if strings.Contains(payload, `"/ops"`) {
		t.Fatalf("user-button must not offer a role-gated app the user does not hold: %s", payload)
	}
	if !strings.Contains(payload, `"applications"`) {
		t.Fatalf("user-button labels must carry the Applications entry")
	}
}

// TestOneAppMeansNoApplicationsMenu: a submenu that offers a single
// destination is a click leading where one already is. The organisations
// submenu has always worked that way (more than one membership, or nothing);
// the applications one now matches.
func TestOneAppMeansNoApplicationsMenu(t *testing.T) {
	mux, _, st := mfaSetup(t)
	ctx := context.Background()
	// Two routes, ONE of them reachable: the role-gated one does not count,
	// which is the case that makes "how many are there" a live question.
	for _, rt := range []store.Route{
		uiRoute("shop", 1, false, ""),
		uiRoute("ops", 2, true, "operations"),
	} {
		if err := st.SaveRoute(ctx, rt); err != nil {
			t.Fatalf("SaveRoute %s: %v", rt.ID, err)
		}
	}
	seedCatalogue(t, st, store.PortalModeLinks, "shop", "ops")
	login := do(t, mux, "POST", "/login", url.Values{"username": {"admin"}, "password": {"s3cret"}}, nil)
	sc := sessionCookieOf(login)

	payload := bodyString(do(t, mux, "GET", "/meerkat/user-button.json", nil, sc))
	if strings.Contains(payload, `"apps"`) {
		t.Fatalf("one reachable app must not produce an applications submenu: %s", payload)
	}

	// A second one appears: the choice is real, the submenu comes back. Both
	// halves are needed now - the route exists AND somebody put it in the
	// catalogue - which is the point: a new route no longer walks into a menu
	// on its own.
	if err := st.SaveRoute(ctx, uiRoute("intranet", 3, true, "")); err != nil {
		t.Fatalf("SaveRoute: %v", err)
	}
	seedCatalogue(t, st, store.PortalModeLinks, "shop", "ops", "intranet")
	payload = bodyString(do(t, mux, "GET", "/meerkat/user-button.json", nil, sc))
	if !strings.Contains(payload, `"apps"`) ||
		!strings.Contains(payload, `"/shop"`) || !strings.Contains(payload, `"/intranet"`) {
		t.Fatalf("two reachable apps must be offered: %s", payload)
	}
}

// TestTheModeDecidesWhatTheMenuOffers pins the three renderings of one list:
// none offers nothing, links offers the catalogue, portal offers exactly one
// way back in - because on a portal installation the bar IS the navigation,
// and a built-in page only needs a door.
func TestTheModeDecidesWhatTheMenuOffers(t *testing.T) {
	mux, _, st := mfaSetup(t)
	ctx := context.Background()
	for _, rt := range []store.Route{
		uiRoute("shop", 1, false, ""),
		uiRoute("intranet", 2, true, ""),
	} {
		if err := st.SaveRoute(ctx, rt); err != nil {
			t.Fatalf("SaveRoute %s: %v", rt.ID, err)
		}
	}
	login := do(t, mux, "POST", "/login", url.Values{"username": {"admin"}, "password": {"s3cret"}}, nil)
	sc := sessionCookieOf(login)

	// The profile hub is the probe: it offers the way back in, in every mode.
	// The user button is NOT, because in portal mode it deliberately carries
	// no submenu at all - the bar is the navigation by then.
	hub := func() string { return bodyString(do(t, mux, "GET", "/profile", nil, sc)) }

	seedCatalogue(t, st, store.PortalModeNone, "shop", "intranet")
	if body := hub(); strings.Contains(body, `href="/shop"`) || strings.Contains(body, `href="/intranet"`) {
		t.Errorf("mode none must offer no application at all:\n%.600s", body)
	}

	seedCatalogue(t, st, store.PortalModeLinks, "shop", "intranet")
	if body := hub(); !strings.Contains(body, `href="/shop"`) || !strings.Contains(body, `href="/intranet"`) {
		t.Errorf("mode links must offer the catalogue:\n%.600s", body)
	}

	seedCatalogue(t, st, store.PortalModePortal, "shop", "intranet")
	body := hub()
	if !strings.Contains(body, `href="/shop"`) {
		t.Errorf("mode portal must still offer the way back in:\n%.600s", body)
	}
	if strings.Contains(body, `href="/intranet"`) {
		t.Errorf("mode portal must offer ONE link, not the whole catalogue:\n%.600s", body)
	}
}

// TestTheCatalogueOrderIsTheMenuOrder: the list is offered in the order
// somebody chose, NOT in routing order - which exists for "first match wins"
// and means nothing to a reader.
func TestTheCatalogueOrderIsTheMenuOrder(t *testing.T) {
	mux, _, st := mfaSetup(t)
	ctx := context.Background()
	for _, rt := range []store.Route{
		uiRoute("shop", 1, false, ""),
		uiRoute("intranet", 2, false, ""),
	} {
		if err := st.SaveRoute(ctx, rt); err != nil {
			t.Fatalf("SaveRoute %s: %v", rt.ID, err)
		}
	}
	// Routing order says shop first; the catalogue says otherwise.
	seedCatalogue(t, st, store.PortalModeLinks, "intranet", "shop")
	login := do(t, mux, "POST", "/login", url.Values{"username": {"admin"}, "password": {"s3cret"}}, nil)
	payload := bodyString(do(t, mux, "GET", "/meerkat/user-button.json", nil, sessionCookieOf(login)))
	if strings.Index(payload, `"/intranet"`) > strings.Index(payload, `"/shop"`) {
		t.Errorf("the catalogue order must win over the routing order: %s", payload)
	}
}
