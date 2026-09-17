package store

import "testing"

func portalRoutes() []Route {
	return []Route{
		{ID: "a", Enabled: true, IsUI: true},
		{ID: "b", Enabled: true, IsUI: true},
		{ID: "c", Enabled: true, IsUI: true},
		{ID: "svc", Enabled: true, IsUI: false}, // a service, not UI
		{ID: "off", Enabled: false, IsUI: true}, // a UI route, disabled
	}
}

func TestSanitizePortalConfigDefaults(t *testing.T) {
	cfg := PortalConfig{Enabled: true}
	if err := SanitizePortalConfig(&cfg, portalRoutes()); err != nil {
		t.Fatalf("empty layout/side should default, got %v", err)
	}
	if cfg.Layout != PortalHeader {
		t.Errorf("empty layout should default to %q, got %q", PortalHeader, cfg.Layout)
	}
	if cfg.Side != "left" {
		t.Errorf("empty side should default to left, got %q", cfg.Side)
	}
}

func TestSanitizePortalConfigUnknownLayoutNamesAllowed(t *testing.T) {
	cfg := PortalConfig{Layout: "sidebar"}
	err := SanitizePortalConfig(&cfg, portalRoutes())
	if err == nil {
		t.Fatal("an unknown layout must be refused")
	}
	// A refusal one cannot act on is a bug: it must name what is allowed.
	for _, want := range []string{PortalHeader, PortalRail} {
		if !containsSub(err.Error(), want) {
			t.Errorf("error %q should name allowed layout %q", err, want)
		}
	}
}

func TestSanitizePortalConfigDisplay(t *testing.T) {
	cfg := PortalConfig{Layout: PortalHeader}
	if err := SanitizePortalConfig(&cfg, portalRoutes()); err != nil || cfg.Display != PortalDisplayBoth {
		t.Fatalf("empty display should default to %q, got %q (err %v)", PortalDisplayBoth, cfg.Display, err)
	}
	bad := PortalConfig{Layout: PortalHeader, Display: "glyphs"}
	err := SanitizePortalConfig(&bad, portalRoutes())
	if err == nil {
		t.Fatal("an unknown display must be refused")
	}
	for _, want := range []string{PortalDisplayBoth, PortalDisplayIcon, PortalDisplayLabel} {
		if !containsSub(err.Error(), want) {
			t.Errorf("error %q should name allowed display %q", err, want)
		}
	}
}

func TestSanitizePortalConfigUnknownSideNamesAllowed(t *testing.T) {
	cfg := PortalConfig{Layout: PortalRail, Side: "up"}
	err := SanitizePortalConfig(&cfg, portalRoutes())
	if err == nil {
		t.Fatal("an unknown side must be refused")
	}
	if !containsSub(err.Error(), "left") || !containsSub(err.Error(), "right") {
		t.Errorf("error %q should name left and right", err)
	}
}

func TestSanitizePortalConfigRejectsUnknownRoute(t *testing.T) {
	cases := map[string]string{
		"absent":  "zzz",
		"service": "svc",
		"off":     "off",
	}
	for name, id := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := PortalConfig{Layout: PortalHeader, Parents: []ModuleParent{{RouteID: id}}}
			if err := SanitizePortalConfig(&cfg, portalRoutes()); err == nil {
				t.Fatalf("route %q (%s) must be refused: not an enabled UI route", id, name)
			}
		})
	}
}

func TestSanitizePortalConfigRejectsEmptyRoute(t *testing.T) {
	cfg := PortalConfig{Layout: PortalHeader, Parents: []ModuleParent{{RouteID: ""}}}
	if err := SanitizePortalConfig(&cfg, portalRoutes()); err == nil {
		t.Fatal("a module with no route must be refused")
	}
	cfg = PortalConfig{Layout: PortalHeader, Parents: []ModuleParent{
		{RouteID: "a", Children: []ModuleChild{{RouteID: ""}}},
	}}
	if err := SanitizePortalConfig(&cfg, portalRoutes()); err == nil {
		t.Fatal("a sub-module with no route must be refused")
	}
}

func TestSanitizePortalConfigDedups(t *testing.T) {
	cfg := PortalConfig{
		Layout: PortalHeader,
		Parents: []ModuleParent{
			{RouteID: "a", Children: []ModuleChild{{RouteID: "b"}, {RouteID: "b"}}},
			{RouteID: "a"}, // duplicate parent
			{RouteID: "c"},
		},
	}
	if err := SanitizePortalConfig(&cfg, portalRoutes()); err != nil {
		t.Fatalf("valid config should pass, got %v", err)
	}
	if len(cfg.Parents) != 2 {
		t.Fatalf("duplicate parent should be dropped, got %d parents", len(cfg.Parents))
	}
	if got := len(cfg.Parents[0].Children); got != 1 {
		t.Errorf("duplicate child should be dropped, got %d children", got)
	}
}

func TestSanitizePortalConfigTrimsAndResolvesIcon(t *testing.T) {
	cfg := PortalConfig{
		Layout: PortalRail,
		Side:   "right",
		Parents: []ModuleParent{
			// A bare name is resolved to its catalogue SVG; text fields trimmed.
			{RouteID: "a", Icon: "  home ", Label: " Home ", Description: " tip "},
		},
	}
	if err := SanitizePortalConfig(&cfg, portalRoutes()); err != nil {
		t.Fatalf("valid config should pass, got %v", err)
	}
	p := cfg.Parents[0]
	if !containsSub(p.Icon, "<svg") {
		t.Errorf("a bare icon name should resolve to an svg, got %q", p.Icon)
	}
	if p.Label != "Home" || p.Description != "tip" {
		t.Errorf("text fields should be trimmed, got label=%q desc=%q", p.Label, p.Description)
	}
	if cfg.Side != "right" {
		t.Errorf("a valid side must be kept, got %q", cfg.Side)
	}
}

// containsSub is a tiny substring helper local to the test (avoids importing
// strings just for this).
func containsSub(s, sub string) bool {
	if sub == "" {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
