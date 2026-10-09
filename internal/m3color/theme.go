package m3color

import (
	"fmt"
	"strconv"
	"strings"
)

// Core is the six colours a Material Theme Builder theme is made from, as hex.
// Primary is the source and the only one required; an empty one is DERIVED
// from it, which is what the builder does with a colour you never touched.
type Core struct {
	Primary        string `json:"primary"`
	Secondary      string `json:"secondary,omitempty"`
	Tertiary       string `json:"tertiary,omitempty"`
	Error          string `json:"error,omitempty"`
	Neutral        string `json:"neutral,omitempty"`
	NeutralVariant string `json:"neutralVariant,omitempty"`
}

// The three contrast levels the builder offers, as the level a scheme takes.
const (
	ContrastStandard = 0.0
	ContrastMedium   = 0.5
	ContrastHigh     = 1.0
)

// Roles are the 49 colour roles of a scheme, in the order the builder's JSON
// export lists them.
var Roles = []string{
	"primary", "surfaceTint", "onPrimary", "primaryContainer", "onPrimaryContainer",
	"secondary", "onSecondary", "secondaryContainer", "onSecondaryContainer",
	"tertiary", "onTertiary", "tertiaryContainer", "onTertiaryContainer",
	"error", "onError", "errorContainer", "onErrorContainer",
	"background", "onBackground", "surface", "onSurface", "surfaceVariant", "onSurfaceVariant",
	"outline", "outlineVariant", "shadow", "scrim", "inverseSurface", "inverseOnSurface", "inversePrimary",
	"primaryFixed", "onPrimaryFixed", "primaryFixedDim", "onPrimaryFixedVariant",
	"secondaryFixed", "onSecondaryFixed", "secondaryFixedDim", "onSecondaryFixedVariant",
	"tertiaryFixed", "onTertiaryFixed", "tertiaryFixedDim", "onTertiaryFixedVariant",
	"surfaceDim", "surfaceBright", "surfaceContainerLowest", "surfaceContainerLow",
	"surfaceContainer", "surfaceContainerHigh", "surfaceContainerHighest",
}

// rolesByName is filled by indexRoles, once the roles exist (see init).
var rolesByName map[string]*dynamicColor

func indexRoles() {
	m := map[string]*dynamicColor{}
	for _, d := range []*dynamicColor{
		background, onBackground, surface, surfaceDim, surfaceBright, surfaceContainerLowest,
		surfaceContainerLow, surfaceContainer, surfaceContainerHigh, surfaceContainerHighest, onSurface,
		surfaceVariant, onSurfaceVariant, inverseSurface, inverseOnSurface, outline, outlineVariant,
		shadow, scrim, surfaceTint, primary, onPrimary, primaryContainer, onPrimaryContainer,
		inversePrimary, secondary, onSecondary, secondaryContainer, onSecondaryContainer, tertiary,
		onTertiary, tertiaryContainer, onTertiaryContainer, errorRole, onError, errorContainer,
		onErrorContainer, primaryFixed, primaryFixedDim, onPrimaryFixed, onPrimaryFixedVariant,
		secondaryFixed, secondaryFixedDim, onSecondaryFixed, onSecondaryFixedVariant, tertiaryFixed,
		tertiaryFixedDim, onTertiaryFixed, onTertiaryFixedVariant,
	} {
		m[d.name] = d
	}
	rolesByName = m
}

// ParseHex reads #rgb or #rrggbb (any case).
func ParseHex(s string) (ARGB, error) {
	h := strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if len(h) != 6 {
		return 0, fmt.Errorf("%q is not a colour: allowed are #rgb and #rrggbb", s)
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("%q is not a colour: allowed are #rgb and #rrggbb", s)
	}
	return ARGB(0xff000000 | uint32(v)), nil
}

// Hex writes the colour the way the builder does: #RRGGBB, upper case.
func (c ARGB) Hex() string { return fmt.Sprintf("#%06X", uint32(c)&0xffffff) }

// Scheme generates one scheme - light or dark, at a contrast level - the way
// Material Theme Builder does: a scheme seeded by the primary gives every
// role, then each colour that was SET replaces its group with the matching
// roles of a scheme seeded by it. That is how the builder can honour a
// secondary of your choosing while keeping the contrast rules of its own: the
// secondary group is the primary group of a scheme whose source is that
// secondary.
//
// colorMatch is the builder's "Color match - stay true to my color inputs":
// the content variant, whose containers keep the tone of the input instead of
// the spec's.
func Scheme(core Core, dark bool, contrast float64, colorMatch bool) (map[string]string, error) {
	v := tonalSpot
	if colorMatch {
		v = content
	}
	seed := func(hex string) (*scheme, error) {
		c, err := ParseHex(hex)
		if err != nil {
			return nil, err
		}
		return newScheme(FromARGB(c), v, dark, contrast), nil
	}
	base, err := seed(core.Primary)
	if err != nil {
		return nil, fmt.Errorf("primary: %w", err)
	}
	out := make(map[string]string, len(Roles))
	for _, r := range Roles {
		out[r] = rolesByName[r].argb(base).Hex()
	}
	take := func(field, hex string, pairs [][2]string) error {
		if hex == "" {
			return nil
		}
		s, err := seed(hex)
		if err != nil {
			return fmt.Errorf("%s: %w", field, err)
		}
		for _, p := range pairs {
			out[p[0]] = rolesByName[p[1]].argb(s).Hex()
		}
		return nil
	}
	group := func(name string, fixed bool) [][2]string {
		up := upper(name)
		pairs := [][2]string{
			{name, "primary"}, {"on" + up, "onPrimary"},
			{name + "Container", "primaryContainer"}, {"on" + up + "Container", "onPrimaryContainer"},
		}
		if fixed {
			pairs = append(pairs,
				[2]string{name + "Fixed", "primaryFixed"}, [2]string{"on" + up + "Fixed", "onPrimaryFixed"},
				[2]string{name + "FixedDim", "primaryFixedDim"}, [2]string{"on" + up + "FixedVariant", "onPrimaryFixedVariant"})
		}
		return pairs
	}
	same := func(roles ...string) [][2]string {
		pairs := make([][2]string, len(roles))
		for i, r := range roles {
			pairs[i] = [2]string{r, r}
		}
		return pairs
	}
	// The builder leaves background and onBackground on the primary's scheme
	// when a neutral is set - they are the 2021 aliases of surface, and it
	// does not move them. Kept, since the point is its output.
	for _, step := range []struct {
		field, hex string
		pairs      [][2]string
	}{
		{"secondary", core.Secondary, group("secondary", true)},
		{"tertiary", core.Tertiary, group("tertiary", true)},
		{"error", core.Error, group("error", false)},
		{"neutral", core.Neutral, same("surface", "onSurface", "shadow", "scrim", "inverseSurface",
			"inverseOnSurface", "surfaceDim", "surfaceBright", "surfaceContainerLowest", "surfaceContainerLow",
			"surfaceContainer", "surfaceContainerHigh", "surfaceContainerHighest")},
		{"neutralVariant", core.NeutralVariant, same("surfaceVariant", "onSurfaceVariant", "outline", "outlineVariant")},
	} {
		if err := take(step.field, step.hex, step.pairs); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// PaletteTones are the tones the builder's export lists for each palette.
var PaletteTones = []int{0, 5, 10, 15, 20, 25, 30, 35, 40, 50, 60, 70, 80, 90, 95, 98, 99, 100}

// Palettes are the five tonal palettes of the builder's export, keyed as it
// keys them. They are NOT the palettes the schemes draw from - the builder
// writes the "content" core palette here whatever the variant - so they are
// reproduced, not reused.
func Palettes(core Core) (map[string]map[string]string, error) {
	contentOf := func(hex string) (h, c float64, err error) {
		v, err := ParseHex(hex)
		if err != nil {
			return 0, 0, err
		}
		hct := FromARGB(v)
		return hct.Hue, hct.Chroma, nil
	}
	h, c, err := contentOf(core.Primary)
	if err != nil {
		return nil, fmt.Errorf("primary: %w", err)
	}
	pal := map[string]*TonalPalette{
		"primary":         paletteOf(h, c),
		"secondary":       paletteOf(h, c/3),
		"tertiary":        paletteOf(h+60, c/2),
		"neutral":         paletteOf(h, min(c/12, 4)),
		"neutral-variant": paletteOf(h, min(c/6, 8)),
	}
	for _, o := range []struct {
		key, field, hex string
		build           func(h, c float64) *TonalPalette
	}{
		{"secondary", "secondary", core.Secondary, paletteOf},
		{"tertiary", "tertiary", core.Tertiary, paletteOf},
		{"neutral", "neutral", core.Neutral, func(h, c float64) *TonalPalette { return paletteOf(h, min(c/12, 4)) }},
		{"neutral-variant", "neutralVariant", core.NeutralVariant, func(h, c float64) *TonalPalette { return paletteOf(h, min(c/6, 8)) }},
	} {
		if o.hex == "" {
			continue
		}
		h, c, err := contentOf(o.hex)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", o.field, err)
		}
		pal[o.key] = o.build(h, c)
	}
	out := map[string]map[string]string{}
	for k, p := range pal {
		tones := map[string]string{}
		for _, t := range PaletteTones {
			tones[strconv.Itoa(t)] = p.Tone(float64(t)).Hex()
		}
		out[k] = tones
	}
	return out, nil
}

// Tone is one tone of the palette a source colour seeds, at the builder's
// tonal-spot chroma - what a theme takes for a colour the spec has no role
// for (the flow pages' glow).
func Tone(hex string, chroma, tone float64) (string, error) {
	v, err := ParseHex(hex)
	if err != nil {
		return "", err
	}
	h := FromARGB(v)
	return paletteOf(h.Hue, chroma).Tone(tone).Hex(), nil
}
