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
