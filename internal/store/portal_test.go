package store

import (
	"strings"
	"testing"
)

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
	cfg := PortalConfig{Mode: PortalModePortal}
	if err := SanitizePortalConfig(&cfg, portalRoutes(), nil); err != nil {
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
	err := SanitizePortalConfig(&cfg, portalRoutes(), nil)
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
	if err := SanitizePortalConfig(&cfg, portalRoutes(), nil); err != nil || cfg.Display != PortalDisplayBoth {
		t.Fatalf("empty display should default to %q, got %q (err %v)", PortalDisplayBoth, cfg.Display, err)
	}
	bad := PortalConfig{Layout: PortalHeader, Display: "glyphs"}
	err := SanitizePortalConfig(&bad, portalRoutes(), nil)
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
	err := SanitizePortalConfig(&cfg, portalRoutes(), nil)
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
	}
	for name, id := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := PortalConfig{Layout: PortalHeader, Entries: []PortalEntry{{RouteID: id}}}
			if err := SanitizePortalConfig(&cfg, portalRoutes(), nil); err == nil {
				t.Fatalf("route %q (%s) must be refused: not a UI route", id, name)
			}
		})
	}
}

// A route turned off is away, not gone: an entry bound to it keeps its place,
// label and icon, and the bar greys it out. Requiring it enabled made one such
// entry refuse every save of the portal, including saves that only added a
// valid one (2026-10-16).
func TestAPortalEntryMayBindATurnedOffRoute(t *testing.T) {
	cfg := PortalConfig{Layout: PortalHeader, Entries: []PortalEntry{
		{RouteID: "off", Label: "Docs"},
		{Label: "Tools", Children: []PortalSubEntry{{RouteID: "off"}, {RouteID: "a"}}},
	}}
	if err := SanitizePortalConfig(&cfg, portalRoutes(), nil); err != nil {
		t.Fatalf("a turned-off UI route was refused: %v", err)
	}
}

// A route deleted, or no longer UI, after its entry was saved is a change made
// ELSEWHERE: the stored reference passes, so the portal can still be saved.
// The same reference ADDED by this write is refused.
func TestAStoredReferenceToAGoneRouteDoesNotBlockTheSave(t *testing.T) {
	stored := PortalConfig{Entries: []PortalEntry{
		{RouteID: "gone"},
		{Label: "Tools", Children: []PortalSubEntry{{RouteID: "svc"}}},
	}}
	cfg := PortalConfig{Layout: PortalHeader, Entries: []PortalEntry{
		{RouteID: "gone"},
		{Label: "Tools", Children: []PortalSubEntry{{RouteID: "svc"}}},
		{RouteID: "b"}, // what this save adds
	}}
	if err := SanitizePortalConfig(&cfg, portalRoutes(), &stored); err != nil {
		t.Fatalf("a save adding a valid entry was refused over stale ones: %v", err)
	}
	added := PortalConfig{Layout: PortalHeader, Entries: []PortalEntry{{RouteID: "zzz"}}}
	if err := SanitizePortalConfig(&added, portalRoutes(), &stored); err == nil || !strings.Contains(err.Error(), "no such route") {
		t.Fatalf("an unknown route added by this save must be refused by name, got %v", err)
	}
}

func TestSanitizePortalConfigRejectsEmptyRoute(t *testing.T) {
	cfg := PortalConfig{Layout: PortalHeader, Entries: []PortalEntry{{RouteID: ""}}}
	if err := SanitizePortalConfig(&cfg, portalRoutes(), nil); err == nil {
		t.Fatal("a module with no route must be refused")
	}
	cfg = PortalConfig{Layout: PortalHeader, Entries: []PortalEntry{
		{RouteID: "a", Children: []PortalSubEntry{{RouteID: ""}}},
	}}
	if err := SanitizePortalConfig(&cfg, portalRoutes(), nil); err == nil {
		t.Fatal("a sub-module with no route must be refused")
	}
}

func TestSanitizePortalConfigDedups(t *testing.T) {
	cfg := PortalConfig{
		Layout: PortalHeader,
		Entries: []PortalEntry{
			{Label: "Tools", Children: []PortalSubEntry{{RouteID: "b"}, {RouteID: "b"}}},
			{RouteID: "a"},
			{RouteID: "a"}, // duplicate entry
			{RouteID: "c"},
		},
	}
	if err := SanitizePortalConfig(&cfg, portalRoutes(), nil); err != nil {
		t.Fatalf("valid config should pass, got %v", err)
	}
	if len(cfg.Entries) != 3 {
		t.Fatalf("duplicate entry should be dropped, got %d entries", len(cfg.Entries))
	}
	if got := len(cfg.Entries[0].Children); got != 1 {
		t.Errorf("duplicate child should be dropped, got %d children", got)
	}
}

// Two kinds of entry: a module is a route and holds nothing, a container holds
// modules and needs a label of its own.
func TestAnEntryIsAModuleOrAContainer(t *testing.T) {
	for name, e := range map[string]PortalEntry{
		"a module holding sub-modules": {RouteID: "a", Children: []PortalSubEntry{{RouteID: "b"}}},
		"a container without a label":  {Children: []PortalSubEntry{{RouteID: "b"}}},
	} {
		cfg := PortalConfig{Layout: PortalHeader, Entries: []PortalEntry{e}}
		if err := SanitizePortalConfig(&cfg, portalRoutes(), nil); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	cfg := PortalConfig{Layout: PortalHeader, Entries: []PortalEntry{{Label: " Tools ", Children: []PortalSubEntry{{RouteID: "b"}}}}}
	if err := SanitizePortalConfig(&cfg, portalRoutes(), nil); err != nil || !cfg.Entries[0].IsContainer() || cfg.Entries[0].Label != "Tools" {
		t.Errorf("a container: %v %+v", err, cfg.Entries)
	}
}

func TestSanitizePortalConfigTrimsAndResolvesIcon(t *testing.T) {
	cfg := PortalConfig{
		Layout: PortalRail,
		Side:   "right",
		Entries: []PortalEntry{
			// A bare name is resolved to its catalogue SVG; text fields trimmed.
			{RouteID: "a", Icon: "  home ", Label: " Home ", Description: " tip "},
		},
	}
	if err := SanitizePortalConfig(&cfg, portalRoutes(), nil); err != nil {
		t.Fatalf("valid config should pass, got %v", err)
	}
	p := cfg.Entries[0]
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

// TestPortalLogoRadiusHasTwoEnds: the radius is a percentage of the mark's box,
// and past a half the corners meet - more only distorts it. The refusal names
// the two ends rather than clamping in silence.
func TestPortalLogoRadiusHasTwoEnds(t *testing.T) {
	for _, r := range []int{-1, 51} {
		cfg := PortalConfig{Mode: PortalModePortal, LogoRadius: r}
		if err := SanitizePortalConfig(&cfg, nil, nil); err == nil {
			t.Errorf("radius %d was accepted", r)
		} else if !strings.Contains(err.Error(), "circle") {
			t.Errorf("the refusal does not name the ends: %v", err)
		}
	}
	cfg := PortalConfig{Mode: PortalModePortal, LogoRadius: 50, HideLogo: true}
	if err := SanitizePortalConfig(&cfg, nil, nil); err != nil {
		t.Errorf("a circle with the logo out was refused: %v", err)
	}
}

// The notifications entry (NOTIF-05) is offered once, wherever it is placed,
// opens no route and holds nothing; it wears the bell when no icon is chosen.
func TestTheNotificationsEntry(t *testing.T) {
	cfg := PortalConfig{Layout: PortalHeader, Entries: []PortalEntry{
		{RouteID: "a"},
		{Label: "Tools", Children: []PortalSubEntry{{RouteID: "b"}, {Special: PortalSpecialNotifications, Label: " Inbox "}}},
		{Special: PortalSpecialNotifications}, // a second one: dropped
	}}
	if err := SanitizePortalConfig(&cfg, portalRoutes(), nil); err != nil {
		t.Fatalf("the notifications entry was refused: %v", err)
	}
	if len(cfg.Entries) != 2 || len(cfg.Entries[1].Children) != 2 {
		t.Fatalf("the second notifications entry should be dropped, got %+v", cfg.Entries)
	}
	bell := cfg.Entries[1].Children[1]
	if bell.Label != "Inbox" || !strings.Contains(bell.Icon, "<svg") {
		t.Fatalf("the entry keeps its label and wears the bell, got %+v", bell)
	}
	if !cfg.NotificationsEntry() {
		t.Fatal("the catalogue places the entry")
	}
	cfg.Entries[1].Disabled = true
	if cfg.NotificationsEntry() {
		t.Fatal("inside a container turned off, the entry is not placed")
	}
	if cfg.Entries[1].IsContainer() != true || (PortalEntry{Special: PortalSpecialNotifications}).IsContainer() {
		t.Fatal("the notifications entry is no container, its own container is one")
	}

	for name, bad := range map[string]PortalEntry{
		"unknown":  {Special: "weather"},
		"route":    {Special: PortalSpecialNotifications, RouteID: "a"},
		"children": {Special: PortalSpecialNotifications, Children: []PortalSubEntry{{RouteID: "a"}}},
	} {
		cfg := PortalConfig{Layout: PortalHeader, Entries: []PortalEntry{bad}}
		if err := SanitizePortalConfig(&cfg, portalRoutes(), nil); err == nil {
			t.Errorf("%s: a special entry %+v should be refused", name, bad)
		}
	}
}

// The notifications page is the built-in one, or a UI route and a path under
// it; anything else is refused naming why.
func TestTheNotificationsPage(t *testing.T) {
	ok := map[string]NotifyPage{
		"built-in": {},
		"root":     {RouteID: "a"},
		"path":     {RouteID: "a", Path: "/inbox"},
		"fragment": {RouteID: "a", Path: "/#/inbox?unread=1"},
		"off":      {RouteID: "off", Path: "/inbox"},
	}
	for name, np := range ok {
		cfg := PortalConfig{Layout: PortalHeader, NotifyPage: np}
		if err := SanitizePortalConfig(&cfg, portalRoutes(), nil); err != nil {
			t.Errorf("%s: %+v refused: %v", name, np, err)
		}
		if np.RouteID != "" && !strings.HasPrefix(cfg.NotifyPage.Path, "/") {
			t.Errorf("%s: the path should default to /, got %q", name, cfg.NotifyPage.Path)
		}
	}
	bad := map[string]struct {
		np   NotifyPage
		says string
	}{
		"path alone": {NotifyPage{Path: "/inbox"}, "pick the UI route"},
		"no route":   {NotifyPage{RouteID: "zzz"}, "no such route"},
		"service":    {NotifyPage{RouteID: "svc"}, "not a UI route"},
		"relative":   {NotifyPage{RouteID: "a", Path: "inbox"}, "must start with /"},
		"other host": {NotifyPage{RouteID: "a", Path: "//evil.test/x"}, "another host"},
		"out":        {NotifyPage{RouteID: "a", Path: "/x/../../admin"}, "leaves the route"},
		"space":      {NotifyPage{RouteID: "a", Path: "/in box"}, "no space"},
		"quote":      {NotifyPage{RouteID: "a", Path: `/x"onclick`}, "quote"},
	}
	for name, c := range bad {
		cfg := PortalConfig{Layout: PortalHeader, NotifyPage: c.np}
		err := SanitizePortalConfig(&cfg, portalRoutes(), nil)
		if err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("%s: %+v should be refused saying %q, got %v", name, c.np, c.says, err)
		}
	}
	// A route gone since it was chosen does not block the save.
	stored := PortalConfig{NotifyPage: NotifyPage{RouteID: "gone", Path: "/inbox"}}
	cfg := PortalConfig{Layout: PortalHeader, NotifyPage: stored.NotifyPage}
	if err := SanitizePortalConfig(&cfg, portalRoutes(), &stored); err != nil {
		t.Fatalf("a stored page whose route is gone blocked the save: %v", err)
	}
}
