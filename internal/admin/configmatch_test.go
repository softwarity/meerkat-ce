package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/softwarity/meerkat/internal/config"
	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

// On a gateway that wears a logo, a configuration captured from what runs IS
// what runs - from every door that asks. The agent's listing fingerprinted the
// running state pictures out, a capture saved it pictures in, and the two
// never met: list_configurations answered matchesRunning false right after
// save_configuration, and the tape never said which point a name was taken
// from.
func TestACaptureMatchesWhatRunsWithALogo(t *testing.T) {
	f := setupBare(t)
	ctx := mcpCtx(rootUser(t, f))

	b := store.DefaultBranding()
	b.AppName = "Acme"
	b.Logo = pngPixel
	if err := store.SanitizeBranding(&b); err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	if err := f.api.st.SetSetting(ctx, store.SettingBranding, b); err != nil {
		t.Fatalf("put branding: %v", err)
	}
	// A page saved the console's way: every ui block present, the ones that
	// are off still holding their settings.
	if err := f.api.st.SaveRoute(ctx, store.Route{
		ID: "app", Name: "app", Enabled: true, IsUI: true, Upstream: "http://x.internal",
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/app/**"}}}},
		UI: &store.RouteUI{
			Roles:      &store.RolesConfig{Mechanism: "class", Tag: "body"},
			UserButton: store.UserButton{Height: 24, Position: "top-right", Shape: "round", PadX: 12, PadY: 12},
		},
	}); err != nil {
		t.Fatalf("seed a route: %v", err)
	}
	if _, err := config.Record(ctx, f.api.st, "root", "seeded"); err != nil {
		t.Fatalf("record a point: %v", err)
	}

	matches := func() map[string]bool {
		t.Helper()
		out, err := f.api.toolListConfigurations(ctx, nil)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		var list struct {
			Configurations []struct {
				Name           string `json:"name"`
				MatchesRunning bool   `json:"matchesRunning"`
			} `json:"configurations"`
		}
		if err := json.Unmarshal([]byte(jsonOf(t, out)), &list); err != nil {
			t.Fatal(err)
		}
		got := map[string]bool{}
		for _, c := range list.Configurations {
			got[c.Name] = c.MatchesRunning
		}
		return got
	}

	// The agent's capture, then a refresh of it, then the console's.
	if _, err := f.api.toolSaveConfiguration(ctx, json.RawMessage(`{"name":"agent"}`)); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := f.api.toolSaveConfiguration(ctx, json.RawMessage(`{"name":"agent","replace":true}`)); err != nil {
		t.Fatalf("save again: %v", err)
	}
	if code, body := f.call(t, "POST", "/api/configurations", `{"name":"console"}`, f.rootC); code != http.StatusCreated {
		t.Fatalf("capture: %d %s", code, body)
	}
	for name, ok := range matches() {
		if !ok {
			t.Errorf("%s was captured from what runs and does not match it", name)
		}
	}

	// The console's current card agrees.
	code, body := f.call(t, "GET", "/api/configurations/current", "", f.rootC)
	if code != http.StatusOK {
		t.Fatalf("current: %d %s", code, body)
	}
	var current struct {
		SavedAs *struct{ Name string } `json:"savedAs"`
	}
	if err := json.Unmarshal([]byte(body), &current); err != nil {
		t.Fatal(err)
	}
	if current.SavedAs == nil {
		t.Errorf("the current card reads as not saved: %s", body)
	}

	// And the tape says which name the running point was saved under.
	code, body = f.call(t, "GET", "/api/config/history", "", f.rootC)
	if code != http.StatusOK {
		t.Fatalf("history: %d %s", code, body)
	}
	var points []struct {
		SavedAs string `json:"savedAs"`
		Live    bool   `json:"live"`
	}
	if err := json.Unmarshal([]byte(body), &points); err != nil {
		t.Fatal(err)
	}
	named := false
	for _, p := range points {
		if p.Live && p.SavedAs != "" {
			named = true
		}
	}
	if !named {
		t.Errorf("the point of the running state names no saved configuration: %s", body)
	}

	// A change is still a change: another logo, and none of them match.
	b.Logo = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAYAAABytg0kAAAAEklEQVR4nGP4z8DwHwyhNAMDAE6cB/k0YFmhAAAAAElFTkSuQmCC"
	if err := f.api.st.SetSetting(context.Background(), store.SettingBranding, b); err != nil {
		t.Fatalf("put branding: %v", err)
	}
	for name, ok := range matches() {
		if ok {
			t.Errorf("%s still matches after the logo changed", name)
		}
	}
}
