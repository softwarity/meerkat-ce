package store

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/softwarity/meerkat/internal/m3color"
)

// Theme styles the SHARED flow pages only (login, select-tenant, OTP, password
// pages - THEME-01/04): global level, never per tenant, and the admin console
// keeps its own look. Several themes coexist and exactly one is active -
// duplicate, tweak, preview, activate, roll back (the CFG-02 philosophy).
//
// A theme is made the way Material Theme Builder makes one: six source colours
// (only the primary required), a contrast level, and the builder's "color
// match" switch. Both schemes - every Material 3 role, light and dark - are
// GENERATED from them on save (m3color), so the two can never disagree and a
// theme exported from the builder is the same theme here. Dark and Light are
// that output, stored so that every reader (the pages, the mails, the portal
// bar) reads colours and not an algorithm; the pages emit them as one token
// block using CSS light-dark(), so the visitor's scheme picks the palette.
//
// There is no other kind of theme. One that arrives as tokens - a palette
// typed before the six colours, an old configuration file, an old client -
// is turned into its colours on the way in (FromTokens) and stored like any
// other; the themes already stored that way were converted by migration v77.
type Theme struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
	// Flat turns off the decorative "Sentinel's Watch" effects on the flow
	// pages (THEME-04): the logo/button/status glows, the ambient primary
	// radial, and the app-name gradient. One switch drives them all through the
	// --mk-glow token (1 = full effects, 0 = flat) - see CSS() and auth.flowTop.
	Flat bool `json:"flat"`
	// Colors are the source colours. An empty one other than the primary is
	// derived from the primary, as the builder derives a colour nobody set.
	Colors m3color.Core `json:"colors,omitzero"`
	// Contrast is standard, medium or high - the builder's three levels,
	// applied to both schemes.
	Contrast string `json:"contrast,omitempty"`
	// ColorMatch is the builder's "stay true to my color inputs": containers
	// keep the tone of the colours given instead of the spec's.
	ColorMatch bool              `json:"colorMatch,omitempty"`
	Dark       map[string]string `json:"dark"`
	Light      map[string]string `json:"light"`
	CreatedAt  int64             `json:"createdAt"`
	UpdatedAt  int64             `json:"updatedAt"`
	// Rev is the revision this row was READ at, carried back by a save so a
	// write built on a version somebody has replaced is refused. Zero means "I
	// read no version" and still wins - see rev.go.
	Rev int64 `json:"rev,omitempty"`
}

// The contrast levels, named as the builder names them.
const (
	ContrastStandard = "standard"
	ContrastMedium   = "medium"
	ContrastHigh     = "high"
)

var contrastLevel = map[string]float64{
	ContrastStandard: m3color.ContrastStandard,
	ContrastMedium:   m3color.ContrastMedium,
	ContrastHigh:     m3color.ContrastHigh,
}

// GlowToken is the one token the Material spec has no role for: the tint of
// the flow pages' ambient glow and of a dialog's backdrop. A deep tint of the
// primary in the dark scheme, a pale one in the light - console/src/app/
// theme/m3.ts makes it the same way, for the preview.
const GlowToken = "night"

// Generated reports whether the theme is made from source colours, as opposed
// to a palette typed token by token before they existed.
func (t Theme) Generated() bool { return t.Colors.Primary != "" }

// ColorsFromTokens reads a theme typed token by token as the colours that make
// the closest generated one: its dark primary as the source, its dark surface
// as the neutral, its light error, its dark muted text as the neutral variant
// - with Color match, which keeps the source's own chroma instead of the
// spec's. Measured against Understory and the old built-in palettes, that rule
// came closest of the ones tried. ok is false when no primary can be read.
func ColorsFromTokens(dark, light map[string]string) (c m3color.Core, ok bool) {
	pick := func(vs ...string) string {
		for _, v := range vs {
			if argb, err := m3color.ParseHex(v); err == nil {
				return strings.ToLower(argb.Hex())
			}
		}
		return ""
	}
	c = m3color.Core{
		Primary:        pick(dark["primary"], light["primary"]),
		Neutral:        pick(dark["surface"], light["surface"]),
		Error:          pick(light["error"], dark["error"]),
		NeutralVariant: pick(dark["onSurfaceVariant"], light["onSurfaceVariant"]),
	}
	return c, c.Primary != ""
}

// Normalize makes the theme what the store keeps: its colours, normalized,
// and both schemes generated from them. A theme that came as tokens only is
// first turned into colours (ColorsFromTokens).
func (t *Theme) Normalize() error {
	if !t.Generated() {
		c, ok := ColorsFromTokens(t.Dark, t.Light)
		if !ok {
			return fmt.Errorf("theme colours: a primary colour is required (colors.primary, #rgb or #rrggbb)")
		}
		t.Colors, t.ColorMatch, t.Contrast = c, true, ""
	}
	return t.Generate()
}

// ThemeTokenKeys returns every colour token a theme carries: the 49 Material
// roles, in the builder's order, then the glow.
func ThemeTokenKeys() []string { return append(slices.Clone(m3color.Roles), GlowToken) }

// ThemeCSSVar is the custom property a token is emitted as: --mk- and the
// role in kebab case (surfaceContainerHigh -> --mk-surface-container-high).
func ThemeCSSVar(key string) string {
	var b strings.Builder
	b.WriteString("--mk-")
	for _, r := range key {
		if r >= 'A' && r <= 'Z' {
			b.WriteByte('-')
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Branding is the application identity shown on the flow pages (THEME-02):
// ONE per gateway (global), whatever theme is active - themes are color
// trials, the identity does not fork with them. Logo is a data URI ("" = the
// built-in meerkat mark).
//
// Favicon is the browser-tab icon and is OPTIONAL on purpose: left empty, the
// logo serves as the icon. A logo is nearly always usable as one, and asking
// for a second image to see one's own mark in the tab is a step most people
// skip - after which the sign-in page of their application wears Meerkat's
// sentinel, which is the one place it must not.
type Branding struct {
	AppName string `json:"appName"`
	Tagline string `json:"tagline"`
	Logo    string `json:"logo"`
	// LogoSize is how big that mark is drawn: "" (normal), "large" or
	// "xlarge". A logo is not a fixed shape - a square sentinel and a wide
	// wordmark do not fill the same box, and the wordmark laid in the square
	// one comes out a third of its height. The alternative was to guess from
	// the image's aspect ratio, which decides for the integrator on a page
	// that is theirs. Three sizes, chosen once, next to the picture.
	LogoSize   string     `json:"logoSize,omitempty"`
	Favicon    string     `json:"favicon,omitempty"`
	Background Background `json:"background,omitzero"`
	// HideMark removes the "powered by Meerkat" line from the served pages.
	// A CHOICE, not a side effect of holding a licence: the white-label
	// feature grants the right to make it, and an installation that never
	// asked keeps the mark. Ignored - and refused on save - without it.
	HideMark bool `json:"hideMark,omitempty"`
}

// The two sizes past the normal one. Named here because the served pages turn
// them into a class and the console offers them as a choice, and a third
// spelling would be a size that saves and does nothing.
const (
	LogoLarge  = "large"
	LogoXLarge = "xlarge"
)

// Background is the image behind the built-in pages (THEME-06). It belongs to
// the BRANDING and not to a theme on purpose: a photograph of a building or a
// product shot is the application's identity, and it must survive the colour
// trials a theme is. Dim is how much of the surface colour is laid over it -
// without it, any picture with a bright corner makes the sign-in card
// unreadable in one scheme or the other, and the fix would be to edit the
// image. Fit says how it meets a screen it was not cut for.
type Background struct {
	// The LIGHT-scheme background, and the one shown in BOTH schemes when Both.
	Image string `json:"image,omitempty"` // data URI, "" = no background
	Fit   string `json:"fit,omitempty"`   // cover (default) | contain | tile
	Dim   int    `json:"dim,omitempty"`   // 0..100, percent of surface laid over
	// Both uses the light background for the dark scheme too, so a single
	// picture covers both; the dark fields below are then ignored (and the
	// console disables their controls). A photograph often reads on one scheme
	// and not the other, so the dark scheme gets its OWN image, fit and dim
	// when Both is off.
	// No omitempty: false is a decision ("each scheme has its own picture"), and
	// omitting it would let a reader's default - which is true - overrule it.
	Both      bool   `json:"both"`
	ImageDark string `json:"imageDark,omitempty"`
	FitDark   string `json:"fitDark,omitempty"`
	DimDark   int    `json:"dimDark,omitempty"`
}

// TabIcon is what a browser tab must show: the favicon when one was set, the
// logo otherwise, and "" when neither exists - the caller then falls back to
// Meerkat's own mark.
func (b Branding) TabIcon() string {
	if b.Favicon != "" {
		return b.Favicon
	}
	return b.Logo
}

// MeerkatBranding is Meerkat's own identity - the ADMIN plane wears it,
// immutably.
func MeerkatBranding() Branding {
	return Branding{AppName: "MEERKAT", Tagline: "The sentinel at your application's door"}
}

// DefaultBranding is the DATA plane's seed: obvious placeholders - name AND
// description - so the integrator understands both are theirs to set (the
// generic logo placeholder is built into the pages).
func DefaultBranding() Branding {
	return Branding{AppName: "MY APP", Tagline: "My application description"}
}

// SanitizeBranding normalizes and validates: an image must be a data URI of
// reasonable size - it lands in a src attribute, nothing else may.
func SanitizeBranding(b *Branding) error {
	b.AppName = strings.TrimSpace(b.AppName)
	b.Tagline = strings.TrimSpace(b.Tagline)
	if b.AppName == "" {
		b.AppName = DefaultBranding().AppName
	}
	if err := checkImageDataURI("logo", b.Logo, 300_000); err != nil {
		return err
	}
	// The normal size is stored as nothing: it is what every branding written
	// before this field said, and what an export should keep saying.
	switch b.LogoSize {
	case "", "normal":
		b.LogoSize = ""
	case LogoLarge, LogoXLarge:
	default:
		return fmt.Errorf("branding logo size %q: allowed are normal, %s, %s", b.LogoSize, LogoLarge, LogoXLarge)
	}
	// A favicon is a 32-pixel square: what does not fit in 64 KiB is a full
	// image someone dropped in by mistake, and it would ride on every page.
	if err := checkImageDataURI("favicon", b.Favicon, 64_000); err != nil {
		return err
	}
	// A full-screen photograph, so a wider budget than a logo - but it is
	// fetched by URL, cached, and never inlined in a page (see the background
	// endpoint), which is what keeps a sign-in page light.
	bg := &b.Background
	if err := checkImageDataURI("background", bg.Image, 1_400_000); err != nil {
		return err
	}
	if err := checkImageDataURI("dark background", bg.ImageDark, 1_400_000); err != nil {
		return err
	}
	fit, err := bgFit(bg.Fit)
	if err != nil {
		return err
	}
	bg.Fit = fit
	fitDark, err := bgFit(bg.FitDark)
	if err != nil {
		return err
	}
	bg.FitDark = fitDark
	bg.Dim = min(100, max(0, bg.Dim))
	bg.DimDark = min(100, max(0, bg.DimDark))
	// One image for both schemes: the dark slot is unused, cleared so it cannot
	// linger in an export or a diff.
	if bg.Both {
		bg.ImageDark, bg.FitDark, bg.DimDark = "", "", 0
	}
	// A layer with no image keeps no fit or dim - there is nothing to apply
	// them to, and the stored shape stays "off" rather than "off with settings".
	if bg.Image == "" {
		bg.Fit, bg.Dim = "", 0
	}
	if bg.ImageDark == "" {
		bg.FitDark, bg.DimDark = "", 0
	}
	// A single picture, no dark counterpart, means "use it in both schemes":
	// the intuitive read of one image, and what every background written before
	// this field said, so an old one keeps showing on a dark page.
	if bg.Image != "" && bg.ImageDark == "" {
		bg.Both = true
	}
	// No image anywhere: back to the zero background, so "no background" is one
	// shape whether Both was on or off.
	if bg.Image == "" && bg.ImageDark == "" {
		b.Background = Background{}
	}
	return nil
}

// bgFit normalizes a background fit to the three it may take, defaulting the
// empty one to cover, and names the allowed set when it is none of them.
func bgFit(fit string) (string, error) {
	switch fit {
	case "", "cover":
		return "cover", nil
	case "contain", "tile":
		return fit, nil
	default:
		return "", fmt.Errorf("branding background fit %q: allowed are cover, contain, tile", fit)
	}
}

// CSS is the page layer that carries the background image: a fixed sheet
// UNDER everything (the grain sits at 2, the card at 3), with the dim laid on
// top of the picture in the same paint - one element, no extra node in a page
// whose markup is shared by every flow screen. Empty when there is no image,
// so a page without one is byte for byte the page it always was.
//
// The image is referenced by URL, never inlined: it is the one asset here that
// can weigh a megabyte, and a data URI would put it in every page of the flow
// instead of once in the browser's cache.
func (b Background) CSS(lightURL, darkURL string) string {
	lightSet := b.Image != ""
	darkImg, darkFit, darkDim, darkU := b.ImageDark, b.FitDark, b.DimDark, darkURL
	if b.Both {
		darkImg, darkFit, darkDim, darkU = b.Image, b.Fit, b.Dim, lightURL
	}
	darkSet := darkImg != ""
	if !lightSet && !darkSet {
		return ""
	}
	lu, du := "", ""
	if lightSet {
		lu = lightURL
	}
	if darkSet {
		du = darkU
	}
	light := bgLayer(lu, b.Fit, b.Dim)
	dark := bgLayer(du, darkFit, darkDim)
	const common = "content: ''; position: fixed; inset: 0; z-index: 0; pointer-events: none; background-position: center;"
	css := fmt.Sprintf("\n    body::before { %s %s }", common, light)
	// One picture for both schemes (Both, or the two happen to match): done.
	if b.Both || dark == light {
		return css
	}
	// light-dark() cannot switch a url(), so the image follows the scheme two
	// ways: the media query for the system preference (an "auto" page), and a
	// body class the server stamps when a scheme is imposed - which outranks the
	// query, so the switcher's choice holds even against the system's.
	css += fmt.Sprintf("\n    @media (prefers-color-scheme: dark) { body::before { %s } }", dark)
	css += fmt.Sprintf("\n    body.mk-scheme-dark::before { %s }", dark)
	css += fmt.Sprintf("\n    body.mk-scheme-light::before { %s }", light)
	return css
}

// bgLayer is the background-* declarations for one scheme: the picture behind
// the optional dim, sized and repeated per its fit. An empty url is "none", so
// a scheme with no image of its own shows nothing rather than the other's.
func bgLayer(url, fit string, dim int) string {
	if url == "" {
		return "background-image: none;"
	}
	size, repeat := "cover", "no-repeat"
	switch fit {
	case "contain":
		size = "contain"
	case "tile":
		size = "auto"
		repeat = "repeat"
	}
	over := ""
	if dim > 0 {
		over = fmt.Sprintf("linear-gradient(color-mix(in srgb, var(--mk-surface) %d%%, transparent), color-mix(in srgb, var(--mk-surface) %d%%, transparent)), ", dim, dim)
	}
	return fmt.Sprintf(`background-image: %surl("%s"); background-size: %s; background-repeat: %s;`, over, url, size, repeat)
}

// imageDataURIPrefixes are the only shapes an image field may take. ICO is
// allowed for the favicon and harmless for the logo: it is the format most
// people already have to hand when they think "favicon".
var imageDataURIPrefixes = []string{
	"data:image/png;base64,",
	"data:image/jpeg;base64,",
	"data:image/webp;base64,",
	"data:image/svg+xml;base64,",
	"data:image/x-icon;base64,",
	"data:image/vnd.microsoft.icon;base64,",
}

func checkImageDataURI(field, value string, limit int) error {
	if value == "" {
		return nil
	}
	if len(value) > limit {
		return fmt.Errorf("branding %s is too large (%d bytes): keep it under ~%d KiB",
			field, len(value), limit*2/3/1024)
	}
	for _, prefix := range imageDataURIPrefixes {
		if strings.HasPrefix(value, prefix) {
			return nil
		}
	}
	return fmt.Errorf("branding %s must be a base64 data URI of type png, jpeg, webp, svg+xml or x-icon", field)
}

// Generate normalizes the source colours and the contrast and writes both
// schemes from them. The colours are stored lower case, like every hex the
// store keeps; the builder's export is upper case, and the console says so.
func (t *Theme) Generate() error {
	c := &t.Colors
	for _, f := range []struct {
		name string
		v    *string
	}{
		{"primary", &c.Primary}, {"secondary", &c.Secondary}, {"tertiary", &c.Tertiary},
		{"error", &c.Error}, {"neutral", &c.Neutral}, {"neutralVariant", &c.NeutralVariant},
	} {
		if *f.v == "" {
			continue
		}
		argb, err := m3color.ParseHex(*f.v)
		if err != nil {
			return fmt.Errorf("theme colour %s: %w", f.name, err)
		}
		*f.v = strings.ToLower(argb.Hex())
	}
	switch t.Contrast {
	case "":
		t.Contrast = ContrastStandard
	case ContrastStandard, ContrastMedium, ContrastHigh:
	default:
		return fmt.Errorf("theme contrast %q: allowed are %s, %s, %s", t.Contrast, ContrastStandard, ContrastMedium, ContrastHigh)
	}
	dark, light, err := schemes(*c, contrastLevel[t.Contrast], t.ColorMatch)
	if err != nil {
		return err
	}
	t.Dark, t.Light = dark, light
	return nil
}

// schemes writes the two palettes of a set of source colours, glow included.
func schemes(c m3color.Core, contrast float64, colorMatch bool) (dark, light map[string]string, err error) {
	out := [2]map[string]string{}
	for i, isDark := range []bool{true, false} {
		s, err := m3color.Scheme(c, isDark, contrast, colorMatch)
		if err != nil {
			return nil, nil, fmt.Errorf("theme colour %w", err)
		}
		tone := 80.0
		if isDark {
			tone = 20
		}
		glow, err := m3color.Tone(c.Primary, 16, tone)
		if err != nil {
			return nil, nil, fmt.Errorf("theme colour primary: %w", err)
		}
		s[GlowToken] = glow
		for k, v := range s {
			s[k] = strings.ToLower(v)
		}
		out[i] = s
	}
	return out[0], out[1], nil
}

// themeSeed is a built-in palette: a name and the colours it is made from.
type themeSeed struct {
	id, name   string
	colors     m3color.Core
	colorMatch bool
}

// presetBase is what every built-in shares - the Catppuccin surface, error and
// muted text they were cut from before they were generated - so only the
// ACCENT changes between them. That is exactly what "different base themes,
// by main colour" means: pick a hue, everything else stays coherent.
//
// They are the colours ColorsFromTokens reads off the palettes the built-ins
// used to be, with its colour match: a gateway that stored one of them before
// v77 holds, after the migration, exactly the built-in it was copied from.
func presetBase(accent string) m3color.Core {
	return m3color.Core{Primary: accent, Neutral: "#24273a", Error: "#d20f39", NeutralVariant: "#a5adcb"}
}

// themeSeeds are the built-in starting palettes (THEME-04): a spread of hues
// on the same base. The first is the default.
var themeSeeds = []themeSeed{
	{"sentinels-watch", "Sentinel's Watch", presetBase("#32d8f4"), true},
	{"midnight", "Midnight", presetBase("#8aadf4"), true},
	{"lavender", "Lavender", presetBase("#b7bdf8"), true},
	{"orchid", "Orchid", presetBase("#c6a0f6"), true},
	{"rose", "Rose", presetBase("#f5bde6"), true},
	{"crimson", "Crimson", presetBase("#ed8796"), true},
	{"ember", "Ember", presetBase("#f5a97f"), true},
	{"forest", "Forest", presetBase("#a6da95"), true},
}

// presets are generated once: a scheme is a few milliseconds of solving, and
// the presets are read on every export and every fallback.
var presets = sync.OnceValue(func() []Theme {
	out := make([]Theme, len(themeSeeds))
	for i, s := range themeSeeds {
		t := Theme{ID: s.id, Name: s.name, Colors: s.colors, ColorMatch: s.colorMatch}
		if err := t.Generate(); err != nil {
			panic("store: built-in theme " + s.id + ": " + err.Error())
		}
		out[i] = t
	}
	return out
})

func (t Theme) clone() Theme {
	t.Dark, t.Light = maps.Clone(t.Dark), maps.Clone(t.Light)
	return t
}

// PresetThemes returns the built-in starting palettes (inactive copies - the
// caller decides activation). The console lists them in its theme picker.
func PresetThemes() []Theme {
	out := make([]Theme, len(presets()))
	for i, p := range presets() {
		out[i] = p.clone()
	}
	return out
}

// DefaultTheme is "The Sentinel's Watch" (the first preset), marked active - the
// fallback whenever no stored theme is available and the admin plane's own skin.
func DefaultTheme() Theme {
	t := presets()[0].clone()
	t.Active = true
	return t
}

// CSS renders the theme as the flow pages' token block: one declaration per
// token via light-dark(), plus the structural tokens. This block is exactly
// what the theme editor manages (THEME-04) - pages never hard-code colors.
func (t Theme) CSS() string {
	def := presets()[0]
	var b strings.Builder
	b.WriteString(":root {\n      color-scheme: light dark;\n")
	for _, key := range ThemeTokenKeys() {
		light, dark := t.Light[key], t.Dark[key]
		if light == "" {
			light = def.Light[key]
		}
		if dark == "" {
			dark = def.Dark[key]
		}
		fmt.Fprintf(&b, "      %s: light-dark(%s, %s);\n", ThemeCSSVar(key), light, dark)
	}
	// --mk-glow scales every decorative effect at once: 1 = full glow, 0 = flat
	// design (the rules in auth.flowTop multiply their blur and color-mix amount
	// by it). Structural, not a color - the flow pages read it, the admin plane
	// (DefaultTheme, Flat=false) always glows.
	glow := "1"
	if t.Flat {
		glow = "0"
	}
	fmt.Fprintf(&b, `      --mk-radius: 16px;
      --mk-radius-small: 10px;
      --mk-font: system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif;
      --mk-mono: ui-monospace, 'SF Mono', 'JetBrains Mono', Menlo, monospace;
      --mk-glow: %s;
    }`, glow)
	return b.String()
}

// seedThemes puts ONE theme in the database: the default, active.
//
// The others are not copied any more. A preset is code (PresetThemes), the
// console offers the whole set in its picker read-only, and duplicating one is
// how a palette of your own begins. Copying all eight at install time made
// eight editable, deletable near-duplicates of things that already existed -
// and then needed a "+" menu whose only job was to put back a preset somebody
// had deleted. Nothing to put back now: they were never removable.
func (s *Store) seedThemes() error {
	ctx := context.Background()
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM themes`).Scan(&n); err != nil {
		return fmt.Errorf("store: count themes: %w", err)
	}
	if n > 0 {
		return nil
	}
	t := DefaultTheme()
	if err := s.SaveTheme(ctx, t); err != nil {
		return fmt.Errorf("store: seed theme %q: %w", t.Name, err)
	}
	return s.ActivateTheme(ctx, t.ID)
}

// SaveTheme inserts or replaces a theme by ID, generated from its colours -
// whatever palettes came with it are the generator's to write, not the
// caller's. A theme that came as tokens only is turned into colours first.
func (s *Store) SaveTheme(ctx context.Context, t Theme) error {
	// See rev.go: a write built on a version somebody has replaced is refused.
	if err := s.checkRev(ctx, "themes", "theme", t.ID, t.Rev); err != nil {
		return err
	}
	if err := t.Normalize(); err != nil {
		return fmt.Errorf("store: theme %q: %w", t.Name, err)
	}
	dj, _ := json.Marshal(t.Dark)
	lj, _ := json.Marshal(t.Light)
	cj, _ := json.Marshal(t.Colors)
	now := time.Now().Unix()
	// A caller may pin created_at (a duplicate inherits its source's, so it sorts
	// right next to it - ListThemes orders by created_at then name); otherwise
	// stamp now. Never overwritten on update (created_at isn't in the SET).
	created := t.CreatedAt
	if created <= 0 {
		created = now
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO themes (id, name, active, flat, colors, contrast, color_match, dark, light, created_at, updated_at, rev)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
		 ON CONFLICT(id) DO UPDATE SET
		   name = excluded.name, flat = excluded.flat, colors = excluded.colors,
		   contrast = excluded.contrast, color_match = excluded.color_match,
		   dark = excluded.dark, light = excluded.light,
		   updated_at = excluded.updated_at, rev = themes.rev + 1`,
		t.ID, t.Name, t.Active, t.Flat, string(cj), t.Contrast, t.ColorMatch, string(dj), string(lj), created, now)
	if err != nil {
		return fmt.Errorf("store: save theme %q: %w", t.Name, err)
	}
	return nil
}

// ActivateTheme makes one theme active and every other inactive, atomically.
func (s *Store) ActivateTheme(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: activate theme: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `UPDATE themes SET active = (id = ?)`, id)
	if err != nil {
		return fmt.Errorf("store: activate theme %q: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store: activate theme %q: %w", id, ErrNoRows)
	}
	var active int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM themes WHERE active = ?`, true).Scan(&active); err != nil {
		return fmt.Errorf("store: activate theme %q: %w", id, err)
	}
	if active != 1 {
		return fmt.Errorf("store: activate theme %q: no such theme", id)
	}
	return tx.Commit()
}

const themeColumns = `id, name, active, flat, colors, contrast, color_match, dark, light, created_at, updated_at, rev`

// GetTheme returns one theme, or an error wrapping sql.ErrNoRows.
func (s *Store) GetTheme(ctx context.Context, id string) (Theme, error) {
	return s.themeRow(ctx, `SELECT `+themeColumns+` FROM themes WHERE id = ?`, id)
}

// GetActiveTheme returns the active theme (there is always exactly one).
func (s *Store) GetActiveTheme(ctx context.Context) (Theme, error) {
	return s.themeRow(ctx, `SELECT `+themeColumns+` FROM themes WHERE active = ?`, true)
}

// completePalettes guarantees every token on the way out: a theme is
// generated whole, so this only covers a row written by a build that knew
// fewer roles - the same holes, filled the same way, in the editor, the
// preview and the emitted CSS.
func completePalettes(t *Theme) {
	if t.Dark == nil {
		t.Dark = map[string]string{}
	}
	if t.Light == nil {
		t.Light = map[string]string{}
	}
	var fill *Theme
	for _, k := range ThemeTokenKeys() {
		if t.Dark[k] != "" && t.Light[k] != "" {
			continue
		}
		if fill == nil {
			g := Theme{Colors: t.Colors, Contrast: t.Contrast, ColorMatch: t.ColorMatch}
			if g.Generate() != nil {
				g = presets()[0]
			}
			fill = &g
		}
		if t.Dark[k] == "" {
			t.Dark[k] = fill.Dark[k]
		}
		if t.Light[k] == "" {
			t.Light[k] = fill.Light[k]
		}
	}
}

func scanTheme(r rowScanner) (Theme, error) {
	var t Theme
	var colors, dark, light string
	if err := r.Scan(&t.ID, &t.Name, &t.Active, &t.Flat, &colors, &t.Contrast, &t.ColorMatch,
		&dark, &light, &t.CreatedAt, &t.UpdatedAt, &t.Rev); err != nil {
		return Theme{}, err
	}
	if colors != "" {
		if err := json.Unmarshal([]byte(colors), &t.Colors); err != nil {
			return Theme{}, fmt.Errorf("store: theme %q: bad source colours: %w", t.ID, err)
		}
	}
	if err := json.Unmarshal([]byte(dark), &t.Dark); err != nil {
		return Theme{}, fmt.Errorf("store: theme %q: bad dark palette: %w", t.ID, err)
	}
	if err := json.Unmarshal([]byte(light), &t.Light); err != nil {
		return Theme{}, fmt.Errorf("store: theme %q: bad light palette: %w", t.ID, err)
	}
	completePalettes(&t)
	return t, nil
}

func (s *Store) themeRow(ctx context.Context, query string, args ...any) (Theme, error) {
	t, err := scanTheme(s.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		return Theme{}, fmt.Errorf("store: get theme: %w", err)
	}
	return t, nil
}

// ListThemes returns every theme in a stable order (creation order, then name)
// that does NOT depend on which one is active.
func (s *Store) ListThemes(ctx context.Context) ([]Theme, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+themeColumns+` FROM themes`)
	if err != nil {
		return nil, fmt.Errorf("store: list themes: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var themes []Theme
	for rows.Next() {
		t, err := scanTheme(rows)
		if err != nil {
			return nil, fmt.Errorf("store: scan theme: %w", err)
		}
		themes = append(themes, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list themes: %w", err)
	}
	// Stable order, INDEPENDENT of which theme is active: activating one must not
	// reshuffle the console's tabs. Creation order, then name as a tiebreak.
	sort.Slice(themes, func(i, j int) bool {
		if themes[i].CreatedAt != themes[j].CreatedAt {
			return themes[i].CreatedAt < themes[j].CreatedAt
		}
		return themes[i].Name < themes[j].Name
	})
	return themes, nil
}

// DeleteTheme removes a theme; the active one is protected.
func (s *Store) DeleteTheme(ctx context.Context, id string) (bool, error) {
	t, err := s.GetTheme(ctx, id)
	if err != nil {
		return false, nil //nolint:nilerr // absent = nothing to delete
	}
	if t.Active {
		return false, fmt.Errorf("store: theme %q is active - activate another theme first", t.Name)
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM themes WHERE id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("store: delete theme %q: %w", id, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
