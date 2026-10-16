package auth

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// TestPortalJSONFiltersByRouteAccess: portal.json offers only the modules a
// caller may open, and drops a parent whose access AND whose children are all
// out of reach - the same rule the apps submenu applied, now two levels deep.
// The access rules themselves never leave the server.
func TestPortalJSONFiltersByRouteAccess(t *testing.T) {
	mux, _, st := mfaSetup(t)
	ctx := context.Background()
	for _, rt := range []store.Route{
		uiRoute("shop", 1, false, ""),             // public parent
		uiRoute("intranet", 2, true, ""),          // authenticated parent
		uiRoute("ops", 3, true, "operations"),     // role-gated (admin lacks it)
		uiRoute("reports", 4, true, "operations"), // role-gated child
	} {
		if err := st.SaveRoute(ctx, rt); err != nil {
			t.Fatalf("SaveRoute %s: %v", rt.ID, err)
		}
	}
	// shop, a module; a container with a reachable module (intranet) and an
	// unreachable one (reports); another container whose only module the
	// caller cannot open; ops, a module the caller cannot open.
	portal := store.PortalConfig{
		Mode: store.PortalModePortal, Layout: store.PortalHeader, Side: "left",
		Entries: []store.PortalEntry{
			{RouteID: "shop"},
			{Label: "Work", Children: []store.PortalSubEntry{{RouteID: "reports"}, {RouteID: "intranet"}}},
			{Label: "Back office", Children: []store.PortalSubEntry{{RouteID: "reports"}}},
			{RouteID: "ops"},
		},
	}
	if err := st.SetSetting(ctx, store.SettingPortal, portal); err != nil {
		t.Fatalf("SetSetting portal: %v", err)
	}

	login := do(t, mux, "POST", "/login", url.Values{"username": {"admin"}, "password": {"s3cret"}}, nil)
	sc := sessionCookieOf(login)

	res := do(t, mux, "GET", "/meerkat/portal.json", nil, sc)
	raw := bodyString(res)

	var payload struct {
		Enabled bool `json:"enabled"`
		Parents []struct {
			Href     string `json:"href"`
			Children []struct {
				Href string `json:"href"`
			} `json:"children"`
		} `json:"parents"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("portal.json is not valid JSON: %v\n%s", err, raw)
	}
	if !payload.Enabled {
		t.Fatal("portal should be enabled")
	}
	// shop, and the Work container. ops is dropped (the caller may not open
	// it), and so is Back office: none of its modules is open to the caller.
	if len(payload.Parents) != 2 {
		t.Fatalf("expected shop and Work, got %d:\n%s", len(payload.Parents), raw)
	}
	if payload.Parents[0].Href != "/shop" || len(payload.Parents[0].Children) != 0 {
		t.Errorf("shop is a module: %+v", payload.Parents[0])
	}
	// In Work only intranet remains, and the container leads to it - the first
	// module the caller may open, not the first one listed.
	work := payload.Parents[1]
	if len(work.Children) != 1 || work.Children[0].Href != "/intranet" || work.Href != "/intranet" {
		t.Errorf("Work should hold /intranet and lead to it, got %+v", work)
	}
	// The security boundary: no access rule leaks to the browser.
	if strings.Contains(raw, "operations") || strings.Contains(raw, "\"roles\"") || strings.Contains(raw, "access") {
		t.Errorf("portal.json must not carry access rules:\n%s", raw)
	}
}

// TestPortalJSONDisabled: with no portal configured, the payload says so and
// lists nothing - the gateway then serves the per-route user buttons.
func TestPortalJSONDisabled(t *testing.T) {
	mux, _, _ := mfaSetup(t)
	res := do(t, mux, "GET", "/meerkat/portal.json", nil, nil)
	var payload struct {
		Enabled bool                `json:"enabled"`
		Parents []map[string]string `json:"parents"`
	}
	if err := json.Unmarshal([]byte(bodyString(res)), &payload); err != nil {
		t.Fatalf("portal.json is not valid JSON: %v", err)
	}
	if payload.Enabled || len(payload.Parents) != 0 {
		t.Fatalf("portal should be off with no parents, got %+v", payload)
	}
}

// TestPortalJSONGreysATurnedOffRoute: an entry whose route is turned off is
// shown in its place, greyed out and leading nowhere; a container whose shown
// modules are all off is greyed out too; an entry turned off in the portal
// itself, or a container with nothing shown, is not there at all.
func TestPortalJSONGreysATurnedOffRoute(t *testing.T) {
	mux, _, st := mfaSetup(t)
	ctx := context.Background()
	off := func(rt store.Route) store.Route { rt.Enabled = false; return rt }
	for _, rt := range []store.Route{
		uiRoute("shop", 1, false, ""),
		off(uiRoute("docs", 2, false, "")),
		off(uiRoute("wiki", 3, false, "")),
		off(uiRoute("ops", 4, true, "operations")), // off AND out of the caller's reach
		uiRoute("intranet", 5, false, ""),
	} {
		if err := st.SaveRoute(ctx, rt); err != nil {
			t.Fatalf("SaveRoute %s: %v", rt.ID, err)
		}
	}
	portal := store.PortalConfig{
		Mode: store.PortalModePortal, Layout: store.PortalHeader, Side: "left",
		Entries: []store.PortalEntry{
			{RouteID: "docs"},
			{RouteID: "shop"},
			{Label: "Library", Children: []store.PortalSubEntry{{RouteID: "wiki"}, {RouteID: "intranet", Disabled: true}}},
			{Label: "Work", Children: []store.PortalSubEntry{{RouteID: "wiki"}, {RouteID: "intranet"}}},
			{Label: "Back office", Children: []store.PortalSubEntry{{RouteID: "ops"}}},
			{RouteID: "intranet", Disabled: true},
		},
	}
	if err := store.SanitizePortalConfig(&portal, mustRoutes(t, st), nil); err != nil {
		t.Fatalf("the portal was refused: %v", err)
	}
	if err := st.SetSetting(ctx, store.SettingPortal, portal); err != nil {
		t.Fatalf("SetSetting portal: %v", err)
	}

	raw := bodyString(do(t, mux, "GET", "/meerkat/portal.json", nil, nil))
	type item struct {
		Label    string `json:"label"`
		Href     string `json:"href"`
		Off      bool   `json:"off"`
		Children []item `json:"children"`
	}
	var payload struct {
		Parents []item `json:"parents"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("portal.json is not valid JSON: %v\n%s", err, raw)
	}
	got := []string{}
	for _, p := range payload.Parents {
		got = append(got, p.Label)
	}
	if strings.Join(got, ",") != "docs,shop,Library,Work" {
		t.Fatalf("expected docs, shop, Library and Work, got %v:\n%s", got, raw)
	}
	docs, shop, library, work := payload.Parents[0], payload.Parents[1], payload.Parents[2], payload.Parents[3]
	if !docs.Off || docs.Href != "" {
		t.Errorf("docs is turned off: greyed out, no address, got %+v", docs)
	}
	if shop.Off || shop.Href != "/shop" {
		t.Errorf("shop is on, got %+v", shop)
	}
	if !library.Off || library.Href != "" || len(library.Children) != 1 || !library.Children[0].Off {
		t.Errorf("Library shows only wiki, off, and is off itself, got %+v", library)
	}
	if work.Off || work.Href != "/intranet" || len(work.Children) != 2 || !work.Children[0].Off || work.Children[1].Off {
		t.Errorf("Work shows wiki greyed and intranet, and leads to intranet, got %+v", work)
	}
}

func mustRoutes(t *testing.T, st *store.Store) []store.Route {
	t.Helper()
	routes, err := st.ListRoutes(context.Background())
	if err != nil {
		t.Fatalf("ListRoutes: %v", err)
	}
	return routes
}
