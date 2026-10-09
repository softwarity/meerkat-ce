package m3color

import "math"

// The 2021 dynamic-colour spec (dynamiccolor/*): each role is a tone of one
// palette, pushed away from the surface behind it until the contrast level
// asks for no more. Only the two variants Material Theme Builder offers are
// carried - tonal spot (its default) and content ("Color match") - so the
// monochrome branches of the original are not.

type variant int

const (
	tonalSpot variant = iota
	content
)

// scheme is DynamicScheme: one source colour, one brightness, one contrast
// level, and the six palettes every role draws from.
type scheme struct {
	source                       HCT
	variant                      variant
	contrast                     float64
	dark                         bool
	primary, secondary, tertiary *TonalPalette
	neutral, neutralVariant      *TonalPalette
	errorPalette                 *TonalPalette
}

func newScheme(source HCT, v variant, dark bool, contrast float64) *scheme {
	s := &scheme{source: source, variant: v, dark: dark, contrast: contrast, errorPalette: paletteOf(25.0, 84.0)}
	switch v {
	case content:
		s.primary = paletteOf(source.Hue, source.Chroma)
		s.secondary = paletteOf(source.Hue, math.Max(source.Chroma-32.0, float64(source.Chroma*0.5)))
		s.tertiary = paletteFromHCT(FromARGB(fixIfDisliked(analogous(source, 3, 6)[2]).ARGB()))
		s.neutral = paletteOf(source.Hue, source.Chroma/8.0)
		s.neutralVariant = paletteOf(source.Hue, source.Chroma/8.0+4.0)
	default:
		s.primary = paletteOf(source.Hue, 36.0)
		s.secondary = paletteOf(source.Hue, 16.0)
		s.tertiary = paletteOf(sanitizeDegreesDouble(source.Hue+60.0), 24.0)
		s.neutral = paletteOf(source.Hue, 6.0)
		s.neutralVariant = paletteOf(source.Hue, 8.0)
	}
	return s
}

func (s *scheme) fidelity() bool { return s.variant == content }

type curve struct{ low, normal, medium, high float64 }

func (c curve) get(level float64) float64 {
	switch {
	case level <= -1.0:
		return c.low
	case level < 0.0:
		return lerp(c.low, c.normal, (level-(-1))/1)
	case level < 0.5:
		return lerp(c.normal, c.medium, (level-0)/0.5)
	case level < 1.0:
		return lerp(c.medium, c.high, (level-0.5)/0.5)
	}
	return c.high
}

type polarity int

const (
	nearer polarity = iota
	lighter
)

type toneDeltaPair struct {
	a, b         *dynamicColor
	delta        float64
	polarity     polarity
	stayTogether bool
}

type dynamicColor struct {
	name             string
	palette          func(*scheme) *TonalPalette
	tone             func(*scheme) float64
	isBackground     bool
	background       func(*scheme) *dynamicColor
	secondBackground func(*scheme) *dynamicColor
	curve            *curve
	pair             func(*scheme) toneDeltaPair
}

func (d *dynamicColor) argb(s *scheme) ARGB { return d.palette(s).Tone(d.getTone(s)) }

func (d *dynamicColor) getTone(s *scheme) float64 {
	decreasing := s.contrast < 0
	if d.pair != nil {
		p := d.pair(s)
		bgTone := d.background(s).getTone(s)
		aIsNearer := p.polarity == nearer || (p.polarity == lighter && !s.dark)
		near, far := p.b, p.a
		if aIsNearer {
			near, far = p.a, p.b
		}
		amNearer := d.name == near.name
		dir := -1.0
		if s.dark {
			dir = 1
		}
		nContrast := near.curve.get(s.contrast)
		fContrast := far.curve.get(s.contrast)
		nTone := near.tone(s)
		if ratioOfTones(bgTone, nTone) < nContrast {
			nTone = foregroundTone(bgTone, nContrast)
		}
		fTone := far.tone(s)
		if ratioOfTones(bgTone, fTone) < fContrast {
			fTone = foregroundTone(bgTone, fContrast)
		}
		if decreasing {
			nTone = foregroundTone(bgTone, nContrast)
			fTone = foregroundTone(bgTone, fContrast)
		}
		if float64((fTone-nTone)*dir) < p.delta {
			fTone = clampDouble(0, 100, nTone+float64(p.delta*dir))
			if float64((fTone-nTone)*dir) < p.delta {
				nTone = clampDouble(0, 100, fTone-float64(p.delta*dir))
			}
		}
		switch {
		case 50 <= nTone && nTone < 60:
			if dir > 0 {
				nTone = 60
				fTone = math.Max(fTone, nTone+float64(p.delta*dir))
			} else {
				nTone = 49
				fTone = math.Min(fTone, nTone+float64(p.delta*dir))
			}
		case 50 <= fTone && fTone < 60:
			switch {
			case p.stayTogether && dir > 0:
				nTone = 60
				fTone = math.Max(fTone, nTone+float64(p.delta*dir))
			case p.stayTogether:
				nTone = 49
				fTone = math.Min(fTone, nTone+float64(p.delta*dir))
			case dir > 0:
				fTone = 60
			default:
				fTone = 49
			}
		}
		if amNearer {
			return nTone
		}
		return fTone
	}

	answer := d.tone(s)
	if d.background == nil {
		return answer
	}
	bgTone := d.background(s).getTone(s)
	desired := d.curve.get(s.contrast)
	if ratioOfTones(bgTone, answer) < desired {
		answer = foregroundTone(bgTone, desired)
	}
	if decreasing {
		answer = foregroundTone(bgTone, desired)
	}
	if d.isBackground && 50 <= answer && answer < 60 {
		if ratioOfTones(49, bgTone) >= desired {
			answer = 49
		} else {
			answer = 60
		}
	}
	if d.secondBackground != nil {
		bgTone1 := d.background(s).getTone(s)
		bgTone2 := d.secondBackground(s).getTone(s)
		upper, lower := math.Max(bgTone1, bgTone2), math.Min(bgTone1, bgTone2)
		if ratioOfTones(upper, answer) >= desired && ratioOfTones(lower, answer) >= desired {
			return answer
		}
		lightOption := lighterTone(upper, desired)
		darkOption := darkerTone(lower, desired)
		available := 0
		var only float64
		if lightOption != -1 {
			available++
			only = lightOption
		}
		if darkOption != -1 {
			available++
			only = darkOption
		}
		if prefersLightForeground(bgTone1) || prefersLightForeground(bgTone2) {
			if lightOption < 0 {
				return 100
			}
			return lightOption
		}
		if available == 1 {
			return only
		}
		if darkOption < 0 {
			return 0
		}
		return darkOption
	}
	return answer
}

func foregroundTone(bgTone, ratio float64) float64 {
	lt := lighterUnsafe(bgTone, ratio)
	dt := darkerUnsafe(bgTone, ratio)
	lr := ratioOfTones(lt, bgTone)
	dr := ratioOfTones(dt, bgTone)
	if prefersLightForeground(bgTone) {
		negligible := math.Abs(lr-dr) < 0.1 && lr < ratio && dr < ratio
		if lr >= ratio || lr >= dr || negligible {
			return lt
		}
		return dt
	}
	if dr >= ratio || dr >= lr {
		return dt
	}
	return lt
}

func prefersLightForeground(tone float64) bool { return jsRound(tone) < 60.0 }

// findDesiredChromaByTone walks the tone until the palette reaches the chroma
// it asks for - the content variant's secondary container.
func findDesiredChromaByTone(hue, chroma, tone float64, byDecreasingTone bool) float64 {
	answer := tone
	closest := NewHCT(hue, chroma, tone)
	if closest.Chroma < chroma {
		peak := closest.Chroma
		for closest.Chroma < chroma {
			if byDecreasingTone {
				answer -= 1.0
			} else {
				answer += 1.0
			}
			potential := NewHCT(hue, chroma, answer)
			if peak > potential.Chroma {
				break
			}
			if math.Abs(potential.Chroma-chroma) < 0.4 {
				break
			}
			if math.Abs(potential.Chroma-chroma) < math.Abs(closest.Chroma-chroma) {
				closest = potential
			}
			peak = math.Max(peak, potential.Chroma)
		}
	}
	return answer
}

func byScheme(dark, light float64) func(*scheme) float64 {
	return func(s *scheme) float64 {
		if s.dark {
			return dark
		}
		return light
	}
}

func byContrast(dark, light curve) func(*scheme) float64 {
	return func(s *scheme) float64 {
		if s.dark {
			return dark.get(s.contrast)
		}
		return light.get(s.contrast)
	}
}

func constant(t float64) func(*scheme) float64 { return func(*scheme) float64 { return t } }

func cv(low, normal, medium, high float64) *curve { return &curve{low, normal, medium, high} }

func neutralP(s *scheme) *TonalPalette        { return s.neutral }
func neutralVariantP(s *scheme) *TonalPalette { return s.neutralVariant }
func primaryP(s *scheme) *TonalPalette        { return s.primary }
func secondaryP(s *scheme) *TonalPalette      { return s.secondary }
func tertiaryP(s *scheme) *TonalPalette       { return s.tertiary }
func errorP(s *scheme) *TonalPalette          { return s.errorPalette }

func highestSurface(s *scheme) *dynamicColor {
	if s.dark {
		return surfaceBright
	}
	return surfaceDim
}

func is(d **dynamicColor) func(*scheme) *dynamicColor {
	return func(*scheme) *dynamicColor { return *d }
}

// The roles, in Material Theme Builder's export order, which is also the
// order the JSON lists them in.
var (
	background, onBackground, surface, surfaceDim, surfaceBright                         *dynamicColor
	surfaceContainerLowest, surfaceContainerLow, surfaceContainer                        *dynamicColor
	surfaceContainerHigh, surfaceContainerHighest, onSurface, surfaceVariant             *dynamicColor
	onSurfaceVariant, inverseSurface, inverseOnSurface, outline, outlineVariant          *dynamicColor
	shadow, scrim, surfaceTint, primary, onPrimary, primaryContainer, onPrimaryContainer *dynamicColor
	inversePrimary, secondary, onSecondary, secondaryContainer, onSecondaryContainer     *dynamicColor
	tertiary, onTertiary, tertiaryContainer, onTertiaryContainer                         *dynamicColor
	errorRole, onError, errorContainer, onErrorContainer                                 *dynamicColor
	primaryFixed, primaryFixedDim, onPrimaryFixed, onPrimaryFixedVariant                 *dynamicColor
	secondaryFixed, secondaryFixedDim, onSecondaryFixed, onSecondaryFixedVariant         *dynamicColor
	tertiaryFixed, tertiaryFixedDim, onTertiaryFixed, onTertiaryFixedVariant             *dynamicColor
)

func init() {
	background = &dynamicColor{name: "background", palette: neutralP, tone: byScheme(6, 98), isBackground: true}
	onBackground = &dynamicColor{name: "onBackground", palette: neutralP, tone: byScheme(90, 10),
		background: is(&background), curve: cv(3, 3, 4.5, 7)}
	surface = &dynamicColor{name: "surface", palette: neutralP, tone: byScheme(6, 98), isBackground: true}
	surfaceDim = &dynamicColor{name: "surfaceDim", palette: neutralP, isBackground: true,
		tone: func(s *scheme) float64 {
			if s.dark {
				return 6
			}
			return curve{87, 87, 80, 75}.get(s.contrast)
		}}
	surfaceBright = &dynamicColor{name: "surfaceBright", palette: neutralP, isBackground: true,
		tone: func(s *scheme) float64 {
			if s.dark {
				return curve{24, 24, 29, 34}.get(s.contrast)
			}
			return 98
		}}
	surfaceContainerLowest = &dynamicColor{name: "surfaceContainerLowest", palette: neutralP, isBackground: true,
		tone: func(s *scheme) float64 {
			if s.dark {
				return curve{4, 4, 2, 0}.get(s.contrast)
			}
			return 100
		}}
	surfaceContainerLow = &dynamicColor{name: "surfaceContainerLow", palette: neutralP, isBackground: true,
		tone: byContrast(curve{10, 10, 11, 12}, curve{96, 96, 96, 95})}
	surfaceContainer = &dynamicColor{name: "surfaceContainer", palette: neutralP, isBackground: true,
		tone: byContrast(curve{12, 12, 16, 20}, curve{94, 94, 92, 90})}
	surfaceContainerHigh = &dynamicColor{name: "surfaceContainerHigh", palette: neutralP, isBackground: true,
		tone: byContrast(curve{17, 17, 21, 25}, curve{92, 92, 88, 85})}
	surfaceContainerHighest = &dynamicColor{name: "surfaceContainerHighest", palette: neutralP, isBackground: true,
		tone: byContrast(curve{22, 22, 26, 30}, curve{90, 90, 84, 80})}
	onSurface = &dynamicColor{name: "onSurface", palette: neutralP, tone: byScheme(90, 10),
		background: highestSurface, curve: cv(4.5, 7, 11, 21)}
	surfaceVariant = &dynamicColor{name: "surfaceVariant", palette: neutralVariantP, tone: byScheme(30, 90), isBackground: true}
	onSurfaceVariant = &dynamicColor{name: "onSurfaceVariant", palette: neutralVariantP, tone: byScheme(80, 30),
		background: highestSurface, curve: cv(3, 4.5, 7, 11)}
	inverseSurface = &dynamicColor{name: "inverseSurface", palette: neutralP, tone: byScheme(90, 20)}
	inverseOnSurface = &dynamicColor{name: "inverseOnSurface", palette: neutralP, tone: byScheme(20, 95),
		background: is(&inverseSurface), curve: cv(4.5, 7, 11, 21)}
	outline = &dynamicColor{name: "outline", palette: neutralVariantP, tone: byScheme(60, 50),
		background: highestSurface, curve: cv(1.5, 3, 4.5, 7)}
	outlineVariant = &dynamicColor{name: "outlineVariant", palette: neutralVariantP, tone: byScheme(30, 80),
		background: highestSurface, curve: cv(1, 1, 3, 4.5)}
	shadow = &dynamicColor{name: "shadow", palette: neutralP, tone: constant(0)}
	scrim = &dynamicColor{name: "scrim", palette: neutralP, tone: constant(0)}
	surfaceTint = &dynamicColor{name: "surfaceTint", palette: primaryP, tone: byScheme(80, 40), isBackground: true}

	primaryPair := func(*scheme) toneDeltaPair { return toneDeltaPair{primaryContainer, primary, 10, nearer, false} }
	primary = &dynamicColor{name: "primary", palette: primaryP, tone: byScheme(80, 40), isBackground: true,
		background: highestSurface, curve: cv(3, 4.5, 7, 7), pair: primaryPair}
	onPrimary = &dynamicColor{name: "onPrimary", palette: primaryP, tone: byScheme(20, 100),
		background: is(&primary), curve: cv(4.5, 7, 11, 21)}
	primaryContainer = &dynamicColor{name: "primaryContainer", palette: primaryP, isBackground: true,
		tone: func(s *scheme) float64 {
			if s.fidelity() {
				return s.source.Tone
			}
			return byScheme(30, 90)(s)
		},
		background: highestSurface, curve: cv(1, 1, 3, 4.5), pair: primaryPair}
	onPrimaryContainer = &dynamicColor{name: "onPrimaryContainer", palette: primaryP,
		tone: func(s *scheme) float64 {
			if s.fidelity() {
				return foregroundTone(primaryContainer.tone(s), 4.5)
			}
			return byScheme(90, 30)(s)
		},
		background: is(&primaryContainer), curve: cv(3, 4.5, 7, 11)}
	inversePrimary = &dynamicColor{name: "inversePrimary", palette: primaryP, tone: byScheme(40, 80),
		background: is(&inverseSurface), curve: cv(3, 4.5, 7, 7)}

	secondaryPair := func(*scheme) toneDeltaPair { return toneDeltaPair{secondaryContainer, secondary, 10, nearer, false} }
	secondary = &dynamicColor{name: "secondary", palette: secondaryP, tone: byScheme(80, 40), isBackground: true,
		background: highestSurface, curve: cv(3, 4.5, 7, 7), pair: secondaryPair}
	onSecondary = &dynamicColor{name: "onSecondary", palette: secondaryP, tone: byScheme(20, 100),
		background: is(&secondary), curve: cv(4.5, 7, 11, 21)}
	secondaryContainer = &dynamicColor{name: "secondaryContainer", palette: secondaryP, isBackground: true,
		tone: func(s *scheme) float64 {
			initial := byScheme(30, 90)(s)
			if !s.fidelity() {
				return initial
			}
			return findDesiredChromaByTone(s.secondary.Hue, s.secondary.Chroma, initial, !s.dark)
		},
		background: highestSurface, curve: cv(1, 1, 3, 4.5), pair: secondaryPair}
	onSecondaryContainer = &dynamicColor{name: "onSecondaryContainer", palette: secondaryP,
		tone: func(s *scheme) float64 {
			if !s.fidelity() {
				return byScheme(90, 30)(s)
			}
			return foregroundTone(secondaryContainer.tone(s), 4.5)
		},
		background: is(&secondaryContainer), curve: cv(3, 4.5, 7, 11)}

	tertiaryPair := func(*scheme) toneDeltaPair { return toneDeltaPair{tertiaryContainer, tertiary, 10, nearer, false} }
	tertiary = &dynamicColor{name: "tertiary", palette: tertiaryP, tone: byScheme(80, 40), isBackground: true,
		background: highestSurface, curve: cv(3, 4.5, 7, 7), pair: tertiaryPair}
	onTertiary = &dynamicColor{name: "onTertiary", palette: tertiaryP, tone: byScheme(20, 100),
		background: is(&tertiary), curve: cv(4.5, 7, 11, 21)}
	tertiaryContainer = &dynamicColor{name: "tertiaryContainer", palette: tertiaryP, isBackground: true,
		tone: func(s *scheme) float64 {
			if !s.fidelity() {
				return byScheme(30, 90)(s)
			}
			return fixIfDisliked(s.tertiary.hct(s.source.Tone)).Tone
		},
		background: highestSurface, curve: cv(1, 1, 3, 4.5), pair: tertiaryPair}
	onTertiaryContainer = &dynamicColor{name: "onTertiaryContainer", palette: tertiaryP,
		tone: func(s *scheme) float64 {
			if !s.fidelity() {
				return byScheme(90, 30)(s)
			}
			return foregroundTone(tertiaryContainer.tone(s), 4.5)
		},
		background: is(&tertiaryContainer), curve: cv(3, 4.5, 7, 11)}

	errorPair := func(*scheme) toneDeltaPair { return toneDeltaPair{errorContainer, errorRole, 10, nearer, false} }
	errorRole = &dynamicColor{name: "error", palette: errorP, tone: byScheme(80, 40), isBackground: true,
		background: highestSurface, curve: cv(3, 4.5, 7, 7), pair: errorPair}
	onError = &dynamicColor{name: "onError", palette: errorP, tone: byScheme(20, 100),
		background: is(&errorRole), curve: cv(4.5, 7, 11, 21)}
	errorContainer = &dynamicColor{name: "errorContainer", palette: errorP, tone: byScheme(30, 90), isBackground: true,
		background: highestSurface, curve: cv(1, 1, 3, 4.5), pair: errorPair}
	onErrorContainer = &dynamicColor{name: "onErrorContainer", palette: errorP, tone: byScheme(90, 30),
		background: is(&errorContainer), curve: cv(3, 4.5, 7, 11)}

	fixedRoles := func(p func(*scheme) *TonalPalette, f, dim, on, onVariant **dynamicColor, name string) {
		pair := func(*scheme) toneDeltaPair { return toneDeltaPair{*f, *dim, 10, lighter, true} }
		*f = &dynamicColor{name: name + "Fixed", palette: p, tone: constant(90), isBackground: true,
			background: highestSurface, curve: cv(1, 1, 3, 4.5), pair: pair}
		*dim = &dynamicColor{name: name + "FixedDim", palette: p, tone: constant(80), isBackground: true,
			background: highestSurface, curve: cv(1, 1, 3, 4.5), pair: pair}
		*on = &dynamicColor{name: "on" + upper(name) + "Fixed", palette: p, tone: constant(10),
			background: is(dim), secondBackground: is(f), curve: cv(4.5, 7, 11, 21)}
		*onVariant = &dynamicColor{name: "on" + upper(name) + "FixedVariant", palette: p, tone: constant(30),
			background: is(dim), secondBackground: is(f), curve: cv(3, 4.5, 7, 11)}
	}
	fixedRoles(primaryP, &primaryFixed, &primaryFixedDim, &onPrimaryFixed, &onPrimaryFixedVariant, "primary")
	fixedRoles(secondaryP, &secondaryFixed, &secondaryFixedDim, &onSecondaryFixed, &onSecondaryFixedVariant, "secondary")
	fixedRoles(tertiaryP, &tertiaryFixed, &tertiaryFixedDim, &onTertiaryFixed, &onTertiaryFixedVariant, "tertiary")
	indexRoles()
}

func upper(s string) string { return string(s[0]-'a'+'A') + s[1:] }
