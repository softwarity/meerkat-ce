package store

import (
	"context"
	"strings"
	"testing"
)

// The flat-design switch (THEME-04) rides on a single CSS token, --mk-glow: the
// flow-page rules multiply every decorative effect by it. Full effects emit 1,
// flat emits 0 - assert both, since the whole feature hangs on this one line.
func TestThemeCSSGlowToken(t *testing.T) {
	glowing := DefaultTheme() // Flat defaults false
	if css := glowing.CSS(); !strings.Contains(css, "--mk-glow: 1;") {
		t.Fatalf("glowing theme CSS must emit --mk-glow: 1, got:\n%s", css)
	}
	flat := DefaultTheme()
	flat.Flat = true
	if css := flat.CSS(); !strings.Contains(css, "--mk-glow: 0;") {
		t.Fatalf("flat theme CSS must emit --mk-glow: 0, got:\n%s", css)
	}
}

// The presets are the "+" menu's starting palettes AND the seed set: assert
// they are complete, distinct, and that the default is the first of them.
func TestPresetThemes(t *testing.T) {
	presets := PresetThemes()
	if len(presets) < 7 {
		t.Fatalf("want at least 7 presets, got %d", len(presets))
	}
	seenID := map[string]bool{}
	seenPrimary := map[string]bool{}
	for _, p := range presets {
		if seenID[p.ID] {
			t.Fatalf("duplicate preset id %q", p.ID)
		}
		seenID[p.ID] = true
		if seenPrimary[p.Dark["primary"]] {
			t.Fatalf("preset %q reuses a dark primary %q", p.ID, p.Dark["primary"])
		}
		seenPrimary[p.Dark["primary"]] = true
		// Every token must be present in both palettes (the base is shared).
		for _, k := range ThemeTokenKeys() {
			if p.Dark[k] == "" || p.Light[k] == "" {
				t.Fatalf("preset %q missing token %q (dark=%q light=%q)", p.ID, k, p.Dark[k], p.Light[k])
			}
		}
		if p.Active {
			t.Fatalf("preset %q must be inactive; activation is the caller's choice", p.ID)
		}
	}
	if def := DefaultTheme(); def.ID != presets[0].ID || !def.Active {
		t.Fatalf("DefaultTheme must be the first preset, active: got id=%q active=%v", def.ID, def.Active)
	}
}

// A fresh store seeds every preset with exactly one active.
func TestSeedThemesInstallsPresets(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	themes, err := s.ListThemes(context.Background())
	if err != nil {
		t.Fatalf("list themes: %v", err)
	}
	if len(themes) != len(PresetThemes()) {
		t.Fatalf("want %d seeded themes, got %d", len(PresetThemes()), len(themes))
	}
	active := 0
	for _, th := range themes {
		if th.Active {
			active++
		}
	}
	if active != 1 {
		t.Fatalf("want exactly one active theme, got %d", active)
	}
}

// The flat flag must survive a save/read round-trip - it is a real column, not
// a transient view concern.
func TestThemeFlatRoundTrips(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	th := DefaultTheme()
	th.ID = "flat-one"
	th.Name = "Flat one"
	th.Active = false
	th.Flat = true
	if err := s.SaveTheme(ctx, th); err != nil {
		t.Fatalf("save flat theme: %v", err)
	}
	got, err := s.GetTheme(ctx, "flat-one")
	if err != nil {
		t.Fatalf("get flat theme: %v", err)
	}
	if !got.Flat {
		t.Fatalf("flat flag lost on round-trip: got %+v", got)
	}

	// And toggling it back off persists too.
	th.Flat = false
	if err := s.SaveTheme(ctx, th); err != nil {
		t.Fatalf("save unflat theme: %v", err)
	}
	got, err = s.GetTheme(ctx, "flat-one")
	if err != nil {
		t.Fatalf("get theme: %v", err)
	}
	if got.Flat {
		t.Fatalf("flat flag should be false after toggle-off: got %+v", got)
	}
}

// TestBackgroundSanitizeAndCSS: a background is an image plus the two settings
// that make it usable - how it meets a screen it was not cut for, and how much
// of the surface colour is laid over it so the card stays readable. Anything
// else is refused rather than emitted into a <style>.
func TestBackgroundSanitizeAndCSS(t *testing.T) {
	const px = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

	b := Branding{AppName: "App", Background: Background{Image: px, Dim: 250}}
	if err := SanitizeBranding(&b); err != nil {
		t.Fatalf("a plain image was refused: %v", err)
	}
	if b.Background.Fit != "cover" {
		t.Errorf("fit should default to cover, got %q", b.Background.Fit)
	}
	if b.Background.Dim != 100 {
		t.Errorf("dim should be clamped to 100, got %d", b.Background.Dim)
	}

	// A fit nobody can render is named, not silently replaced.
	bad := Branding{AppName: "App", Background: Background{Image: px, Fit: "stretch"}}
	if err := SanitizeBranding(&bad); err == nil {
		t.Error("an unknown fit should be refused")
	}

	// No image: the framing that described it goes with it, so an exported
	// configuration never carries settings for a picture that is not there.
	empty := Branding{AppName: "App", Background: Background{Fit: "tile", Dim: 40}}
	if err := SanitizeBranding(&empty); err != nil {
		t.Fatal(err)
	}
	if empty.Background != (Background{}) {
		t.Errorf("a background without an image should be empty, got %+v", empty.Background)
	}
	if empty.Background.CSS("/meerkat/background") != "" {
		t.Error("no image must emit no rule at all")
	}

	css := b.Background.CSS("/meerkat/background")
	for _, want := range []string{"body::before", `url("/meerkat/background")`, "background-size: cover", "var(--mk-surface) 100%"} {
		if !strings.Contains(css, want) {
			t.Errorf("the rule is missing %q:\n%s", want, css)
		}
	}

	tile := Background{Image: px, Fit: "tile"}
	if css := tile.CSS("/x"); !strings.Contains(css, "background-repeat: repeat") {
		t.Errorf("a tiled background must repeat:\n%s", css)
	}
}

// A route saved when the mechanism was a list comes back with the one it
// meant. Normalised on the way out of the store, because the console above
// all must not show "accept" for a route stored as "path" - saving that
// screen would take the mechanism away without anyone touching it.
func TestLocaleMechanismNormalisesOnRead(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	r := Route{
		ID: "r1", Name: "legacy", Enabled: true, Upstream: "http://up", IsUI: true,
		Locales: &LocalesConfig{Mechanisms: []string{"path"}},
	}
	if err := s.SaveRoute(ctx, r); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetRoute(ctx, "r1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Locales.Mechanism != "path" {
		t.Errorf("mechanism = %q, want path", got.Locales.Mechanism)
	}
	if got.Locales.Mechanisms != nil {
		t.Errorf("the retired list came back out: %v", got.Locales.Mechanisms)
	}
}
