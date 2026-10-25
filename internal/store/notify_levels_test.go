package store

import (
	"context"
	"testing"

	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// A text is sent as one string now, stored without a language; what was
// stored by language before keeps reading in the reader's.
func TestPickReadsBothShapes(t *testing.T) {
	if got := (Localized{"": "New station"}).Pick("fr"); got != "New station" {
		t.Fatalf("a text without a language is everybody's: %q", got)
	}
	old := Localized{"fr": "Nouvelle station", "en": "New station"}
	for lang, want := range map[string]string{"fr-CA": "Nouvelle station", "de": "New station", "": "New station"} {
		if got := old.Pick(lang); got != want {
			t.Errorf("Pick(%q) = %q, want %q", lang, got, want)
		}
	}
}

// Three levels decide whether a tenant receives notifications, and each one
// can only take away (NOTIF-05).
func TestEveryLevelMustAllowATenant(t *testing.T) {
	n := NotifySettings{Enabled: true}
	if !n.Effective("a", true) {
		t.Fatal("a tenant nobody turned off must receive: the lists hold the exceptions")
	}
	for _, c := range []struct {
		name string
		n    NotifySettings
		by   string
	}{
		{"switch", NotifySettings{}, NotifyLevelRoot},
		{"root", NotifySettings{Enabled: true, RootOff: []string{"a"}}, NotifyLevelRoot},
		{"app-admin", NotifySettings{Enabled: true, AdminOff: []string{"a"}}, NotifyLevelAppAdmin},
		{"tenant", NotifySettings{Enabled: true, TenantOff: []string{"a"}}, NotifyLevelTenant},
		{"root over the others", NotifySettings{Enabled: true, RootOff: []string{"a"}, AdminOff: []string{"a"}}, NotifyLevelRoot},
	} {
		if c.n.Effective("a", true) || c.n.OffBy("a", true) != c.by {
			t.Errorf("%s: effective %v, off by %q, want off by %q", c.name, c.n.Effective("a", true), c.n.OffBy("a", true), c.by)
		}
	}
	if n.Effective("", true) {
		t.Fatal("no tenant receives nothing")
	}
}

// On a single-tenant gateway the tenant level does not exist: what it said
// before the switch to single is not read.
func TestTheTenantLevelIsIgnoredOnASingleTenant(t *testing.T) {
	n := NotifySettings{Enabled: true, TenantOff: []string{"a"}}
	if !n.Effective("a", false) {
		t.Fatal("single tenant: the tenant level must not count")
	}
	if n.Effective("a", true) {
		t.Fatal("multi tenant: the tenant level must count")
	}
	if s, _ := n.View(NotifyLevelTenant, "a", false); s != NotifyOff {
		t.Fatalf("the tenant level has no view on a single tenant, got %q", s)
	}
}

// Each level sees what it decided, and who below or above decided otherwise.
func TestEachLevelSeesWhoTurnedATenantOff(t *testing.T) {
	n := NotifySettings{Enabled: true, RootOff: []string{"r"}, AdminOff: []string{"m"}, TenantOff: []string{"t"}}
	for _, c := range []struct{ level, tenant, state, by string }{
		{NotifyLevelRoot, "ok", NotifyOn, ""},
		{NotifyLevelRoot, "r", NotifyOff, NotifyLevelRoot},
		{NotifyLevelRoot, "m", NotifyOffBelow, NotifyLevelAppAdmin},
		{NotifyLevelRoot, "t", NotifyOffBelow, NotifyLevelTenant},
		{NotifyLevelAppAdmin, "r", NotifyOffAbove, NotifyLevelRoot},
		{NotifyLevelAppAdmin, "m", NotifyOff, NotifyLevelAppAdmin},
		{NotifyLevelAppAdmin, "t", NotifyOffBelow, NotifyLevelTenant},
		{NotifyLevelTenant, "m", NotifyOffAbove, NotifyLevelAppAdmin},
		{NotifyLevelTenant, "t", NotifyOff, NotifyLevelTenant},
		{NotifyLevelTenant, "ok", NotifyOn, ""},
	} {
		if s, by := n.View(c.level, c.tenant, true); s != c.state || by != c.by {
			t.Errorf("%s on %s: %q by %q, want %q by %q", c.level, c.tenant, s, by, c.state, c.by)
		}
	}
	// Root's switch off: every level under root sees every tenant off above;
	// root still sees its own per-tenant decisions.
	n.Enabled = false
	if s, by := n.View(NotifyLevelAppAdmin, "ok", true); s != NotifyOffAbove || by != NotifyLevelRoot {
		t.Errorf("switch off, app-admin sees %q by %q", s, by)
	}
	if s, _ := n.View(NotifyLevelRoot, "ok", true); s != NotifyOn {
		t.Errorf("switch off, root sees its own decision as %q", s)
	}
}

// Forcing a tenant on resets the levels below; turning it on does not.
func TestForcingResetsTheLevelsBelow(t *testing.T) {
	n := NotifySettings{Enabled: true, RootOff: []string{"a", "b"}, AdminOff: []string{"a", "c"}, TenantOff: []string{"a", "c"}}
	if err := n.Decide(NotifyLevelRoot, nil, []string{"b"}, []string{"a"}); err != nil {
		t.Fatal(err)
	}
	if len(n.RootOff) != 0 || len(n.AdminOff) != 1 || n.AdminOff[0] != "c" || len(n.TenantOff) != 1 {
		t.Fatalf("root forced a: %+v", n)
	}
	if err := n.Decide(NotifyLevelAppAdmin, nil, nil, []string{"c"}); err != nil {
		t.Fatal(err)
	}
	if len(n.AdminOff) != 0 || len(n.TenantOff) != 0 {
		t.Fatalf("app-admin forced c: %+v", n)
	}
	if err := n.Decide(NotifyLevelTenant, []string{"c"}, nil, nil); err != nil || !n.offAt(NotifyLevelTenant, "c", true) {
		t.Fatalf("the tenant admin turns it off again afterwards: %v %+v", err, n)
	}
	if err := n.Decide(NotifyLevelAppAdmin, []string{"x"}, []string{"x"}, nil); err == nil {
		t.Fatal("off and on at once must be refused")
	}
	if err := n.Decide("owner", nil, nil, nil); err == nil {
		t.Fatal("an unknown level must be refused")
	}
}

// The setting written before the levels is read as root's decision: the
// tenants it listed on, the others turned off by root.
func TestTheOldSettingIsReadAsRootsDecision(t *testing.T) {
	s, err := OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()
	if err := s.SaveTenant(ctx, Tenant{ID: "other", Name: "other", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSetting(ctx, SettingNotifications, map[string]any{"tenants": []string{DefaultTenantID}, "cap": 7}); err != nil {
		t.Fatal(err)
	}
	n := s.NotifySettings(ctx)
	if !n.Enabled || !n.Effective(DefaultTenantID, true) || n.Effective("other", true) || n.Cap != 7 {
		t.Fatalf("old setting read as %+v", n)
	}
	if err := s.SetSetting(ctx, SettingNotifications, "not an object"); err != nil {
		t.Fatal(err)
	}
	if n := s.NotifySettings(ctx); n.Enabled || n.Effective(DefaultTenantID, false) {
		t.Fatalf("an unreadable setting must be off, got %+v", n)
	}
}

// What a level finds again when it allows a tenant it had turned off.
func TestBelowByIgnoresTheLevelsOwnDecision(t *testing.T) {
	n := NotifySettings{Enabled: true, RootOff: []string{"a"}, AdminOff: []string{"a"}}
	if got := n.BelowBy(NotifyLevelRoot, "a", true); got != NotifyLevelAppAdmin {
		t.Fatalf("below root: %q", got)
	}
	if got := n.BelowBy(NotifyLevelTenant, "a", true); got != "" {
		t.Fatalf("below the tenant admin: %q", got)
	}
}
