package store

import (
	"context"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/m3color"
	"github.com/softwarity/meerkat/internal/store/dbtest"
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

// A fresh store holds ONE theme, active: the default. The other presets are
// code the console offers read-only, so copying them here would have created
// editable near-duplicates of things that already exist.
func TestSeedThemesInstallsTheDefaultAlone(t *testing.T) {
	s, err := OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	themes, err := s.ListThemes(context.Background())
	if err != nil {
		t.Fatalf("list themes: %v", err)
	}
	if len(themes) != 1 {
		t.Fatalf("want a single seeded theme, got %d", len(themes))
	}
	if !themes[0].Active {
		t.Fatalf("the only theme must be the active one: %+v", themes[0])
	}
	if themes[0].ID != DefaultTheme().ID {
		t.Fatalf("seeded %q, want the default %q", themes[0].ID, DefaultTheme().ID)
	}
}

// The flat flag must survive a save/read round-trip - it is a real column, not
// a transient view concern.
func TestThemeFlatRoundTrips(t *testing.T) {
	s, err := OpenAt(t.TempDir(), dbtest.URL(t))
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
	if empty.Background.CSS("/meerkat/background", "/meerkat/background-dark") != "" {
		t.Error("no image must emit no rule at all")
	}

	// A single image is "use in both schemes": one rule, no dark URL, no
	// per-scheme override to drag in.
	css := b.Background.CSS("/meerkat/background", "/meerkat/background-dark")
	for _, want := range []string{"body::before", `url("/meerkat/background")`, "background-size: cover", "var(--mk-surface) 100%"} {
		if !strings.Contains(css, want) {
			t.Errorf("the rule is missing %q:\n%s", want, css)
		}
	}
	for _, unwanted := range []string{"/meerkat/background-dark", "mk-scheme-dark", "prefers-color-scheme"} {
		if strings.Contains(css, unwanted) {
			t.Errorf("a single (both-schemes) image must not emit %q:\n%s", unwanted, css)
		}
	}

	// Two pictures: the dark one follows the scheme, by the media query for an
	// auto page and by the body class the server stamps for an imposed one.
	two := Background{Image: px, Fit: "cover", ImageDark: px, FitDark: "tile", DimDark: 30}
	if err := SanitizeBranding(&Branding{AppName: "App", Background: two}); err != nil {
		t.Fatal(err)
	}
	twoCSS := two.CSS("/meerkat/background", "/meerkat/background-dark")
	for _, want := range []string{
		`url("/meerkat/background-dark")`, "@media (prefers-color-scheme: dark)",
		"body.mk-scheme-dark::before", "body.mk-scheme-light::before", "background-repeat: repeat",
	} {
		if !strings.Contains(twoCSS, want) {
			t.Errorf("the two-scheme rule is missing %q:\n%s", want, twoCSS)
		}
	}

	tile := Background{Image: px, Fit: "tile"}
	if css := tile.CSS("/x", "/x-dark"); !strings.Contains(css, "background-repeat: repeat") {
		t.Errorf("a tiled background must repeat:\n%s", css)
	}
}

// TestLogoSizeSanitize: the mark's box is one of three, and the normal one is
// stored as nothing - what every branding written before the field said, and
// what an export should keep saying. A fourth spelling is refused rather than
// saved as a class no page dresses.
func TestLogoSizeSanitize(t *testing.T) {
	for _, in := range []string{"", "normal"} {
		b := Branding{AppName: "App", LogoSize: in}
		if err := SanitizeBranding(&b); err != nil {
			t.Fatalf("%q was refused: %v", in, err)
		}
		if b.LogoSize != "" {
			t.Errorf("%q should normalize to the empty size, got %q", in, b.LogoSize)
		}
	}
	for _, in := range []string{LogoLarge, LogoXLarge} {
		b := Branding{AppName: "App", LogoSize: in}
		if err := SanitizeBranding(&b); err != nil {
			t.Fatalf("%q was refused: %v", in, err)
		}
		if b.LogoSize != in {
			t.Errorf("%q should survive, got %q", in, b.LogoSize)
		}
	}
	bad := Branding{AppName: "App", LogoSize: "huge"}
	if err := SanitizeBranding(&bad); err == nil {
		t.Error("an unknown logo size should be refused")
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

// A theme with source colours is GENERATED on save: what the caller sent as
// palettes is replaced by the schemes of its colours, at its contrast, so a
// stored theme can never say one thing in its colours and another in its
// tokens.
func TestAGeneratedThemeIsItsColours(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	th := Theme{ID: "violet", Name: "Violet", Colors: m3color.Core{Primary: "#6750A4", Tertiary: "#B33B15"},
		Contrast: ContrastHigh, Dark: map[string]string{"primary": "#000000"}}
	if err := s.SaveTheme(ctx, th); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetTheme(ctx, "violet")
	if err != nil {
		t.Fatal(err)
	}
	if got.Colors.Primary != "#6750a4" || got.Colors.Tertiary != "#b33b15" || got.Contrast != ContrastHigh {
		t.Fatalf("source colours or contrast lost: %+v %q", got.Colors, got.Contrast)
	}
	for i, isDark := range []bool{true, false} {
		want, err := m3color.Scheme(got.Colors, isDark, m3color.ContrastHigh, false)
		if err != nil {
			t.Fatal(err)
		}
		palette := []map[string]string{got.Dark, got.Light}[i]
		for _, r := range m3color.Roles {
			if palette[r] != strings.ToLower(want[r]) {
				t.Errorf("dark=%v %s = %q, want the generated %q", isDark, r, palette[r], want[r])
			}
		}
		if palette[GlowToken] == "" {
			t.Errorf("dark=%v carries no glow", isDark)
		}
	}
	// A contrast that is not one of the three is named, with the three.
	th.Contrast = "max"
	if err := s.SaveTheme(ctx, th); err == nil || !strings.Contains(err.Error(), "allowed are standard, medium, high") {
		t.Errorf("contrast max: %v", err)
	}
	// A contrast with nothing to apply it to is refused, not silently dropped.
	if err := s.SaveTheme(ctx, Theme{ID: "x", Name: "x", Contrast: ContrastHigh}); err == nil {
		t.Error("a contrast without a primary colour was accepted")
	}
}

// A theme that arrives as tokens - an old client, an old configuration file -
// is stored as the colours that make it: there is no second shape of theme.
func TestATypedThemeIsStoredAsItsColours(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	if err := s.SaveTheme(ctx, Theme{ID: "old", Name: "Old",
		Dark:  map[string]string{"primary": "#32D8F4", "surface": "#24273a", "onSurfaceVariant": "#a5adcb"},
		Light: map[string]string{"primary": "#1a88b0", "error": "#d20f39"}}); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetTheme(ctx, "old")
	if err != nil {
		t.Fatal(err)
	}
	want := m3color.Core{Primary: "#32d8f4", Neutral: "#24273a", Error: "#d20f39", NeutralVariant: "#a5adcb"}
	if got.Colors != want || !got.ColorMatch || got.Contrast != ContrastStandard {
		t.Fatalf("stored as %+v colorMatch=%v contrast=%q, want %+v with colour match", got.Colors, got.ColorMatch, got.Contrast, want)
	}
	// Nothing to read a colour from: refused, with what is required.
	if err := s.SaveTheme(ctx, Theme{ID: "none", Name: "None", Dark: map[string]string{"surface": "nope"}}); err == nil ||
		!strings.Contains(err.Error(), "colors.primary") {
		t.Errorf("a theme with no primary: %v", err)
	}
}

// The upgrade, for real: a row typed token by token, as every gateway before
// v76 wrote them, comes out of migration v77 as its colours and what they make.
func TestMigrationTurnsTypedThemesIntoColours(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO themes (id, name, active, flat, colors, contrast, color_match, dark, light, created_at, updated_at, rev)
		 VALUES ('typed', 'Typed', FALSE, FALSE, '{}', '', FALSE, ?, ?, 1, 1, 1)`,
		`{"primary":"#d3c4b0","surface":"#1b1200","onSurfaceVariant":"#9b8f7a"}`, `{"primary":"#665a48","error":"#8c3a2b"}`); err != nil {
		t.Fatal(err)
	}
	if err := s.db.setSchemaVersion(76); err != nil {
		t.Fatal(err)
	}
	if err := s.runMigrations(migrations, 76); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	got, err := s.GetTheme(ctx, "typed")
	if err != nil {
		t.Fatal(err)
	}
	if got.Colors.Primary != "#d3c4b0" || got.Colors.Neutral != "#1b1200" || !got.ColorMatch {
		t.Fatalf("not converted: %+v colorMatch=%v", got.Colors, got.ColorMatch)
	}
	gen := Theme{Colors: got.Colors, Contrast: got.Contrast, ColorMatch: got.ColorMatch}
	if err := gen.Generate(); err != nil {
		t.Fatal(err)
	}
	for _, k := range ThemeTokenKeys() {
		if got.Dark[k] != gen.Dark[k] || got.Light[k] != gen.Light[k] {
			t.Fatalf("%s is not what the colours make: %s/%s, want %s/%s", k, got.Dark[k], got.Light[k], gen.Dark[k], gen.Light[k])
		}
	}
}

func TestThemeCSSVarIsKebab(t *testing.T) {
	for in, want := range map[string]string{
		"primary": "--mk-primary", "onSurfaceVariant": "--mk-on-surface-variant",
		"surfaceContainerHighest": "--mk-surface-container-highest", "night": "--mk-night",
	} {
		if got := ThemeCSSVar(in); got != want {
			t.Errorf("%s -> %s, want %s", in, got, want)
		}
	}
}

// A gateway that stored a built-in before v77 holds, once migrated, exactly
// the built-in it was copied from: the presets are the colours the conversion
// reads off the palettes they used to be.
func TestAMigratedBuiltInIsTheBuiltIn(t *testing.T) {
	oldSentinel := map[string]map[string]string{
		"dark": {"primary": "#32d8f4", "onPrimary": "#00363f", "night": "#1b2f40", "surface": "#24273a",
			"onSurface": "#cad3f5", "surfaceContainer": "#2a2e42", "surfaceContainerHigh": "#363a4f",
			"onSurfaceVariant": "#a5adcb", "outline": "#494d64", "error": "#ed8796"},
		"light": {"primary": "#1a88b0", "onPrimary": "#ffffff", "night": "#a5c8d4", "surface": "#eff1f5",
			"onSurface": "#4c4f69", "surfaceContainer": "#e6e9ef", "surfaceContainerHigh": "#dce0e8",
			"onSurfaceVariant": "#6c6f85", "outline": "#acb0be", "error": "#d20f39"},
	}
	got := Theme{Dark: oldSentinel["dark"], Light: oldSentinel["light"]}
	if err := got.Normalize(); err != nil {
		t.Fatal(err)
	}
	want := DefaultTheme()
	if got.Colors != want.Colors || got.ColorMatch != want.ColorMatch {
		t.Fatalf("the migrated Sentinel's Watch is %+v (match %v), the built-in %+v (match %v)",
			got.Colors, got.ColorMatch, want.Colors, want.ColorMatch)
	}
}

// A theme's typefaces are families the gateway ships, each in a slot that
// takes its kind; the CSS declares their faces and sets the stacks.
func TestAThemeIsSetInItsFonts(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	th := Theme{ID: "typo", Name: "Typo", Colors: m3color.Core{Primary: "#6750a4"},
		Fonts: ThemeFonts{Display: "Playfair Display", Body: "Inter", Code: "JetBrains Mono"}}
	if err := s.SaveTheme(ctx, th); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetTheme(ctx, "typo")
	if err != nil {
		t.Fatal(err)
	}
	if got.Fonts != th.Fonts {
		t.Fatalf("fonts came back as %+v", got.Fonts)
	}
	css := got.CSS()
	for _, want := range []string{"font-family: 'Playfair Display'", "--mk-font: 'Inter', 'Noto Sans Arabic'",
		"--mk-display: 'Playfair Display'", "--mk-mono: 'JetBrains Mono'", "/meerkat/fonts/inter-latin.woff2?v="} {
		if !strings.Contains(css, want) {
			t.Errorf("the CSS lacks %q", want)
		}
	}
	// No font chosen: the system's, and no display face at all.
	if css := DefaultTheme().CSS(); strings.Contains(css, "@font-face") || strings.Contains(css, "--mk-display") {
		t.Errorf("a theme with no font declares one:\n%s", css)
	}
	th.Fonts.Code = "Inter"
	if err := s.SaveTheme(ctx, th); err == nil || !strings.Contains(err.Error(), "allowed are JetBrains Mono, Roboto Mono") {
		t.Errorf("Inter as the code font: %v", err)
	}
}
