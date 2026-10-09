package m3color

import (
	"math"
	"sort"
)

// TonalPalette is one hue at one chroma, every tone of it on demand
// (palettes/tonal_palette). A scheme holds six, and each role is a tone of one.
type TonalPalette struct {
	Hue, Chroma float64
	cache       map[float64]ARGB
}

func paletteOf(hue, chroma float64) *TonalPalette {
	return &TonalPalette{Hue: hue, Chroma: chroma, cache: map[float64]ARGB{}}
}

func paletteFromHCT(h HCT) *TonalPalette { return paletteOf(h.Hue, h.Chroma) }

// Tone is the palette's colour at that tone.
func (p *TonalPalette) Tone(tone float64) ARGB {
	if c, ok := p.cache[tone]; ok {
		return c
	}
	c := solveToInt(p.Hue, p.Chroma, tone)
	p.cache[tone] = c
	return c
}

func (p *TonalPalette) hct(tone float64) HCT { return FromARGB(p.Tone(tone)) }

// ---- contrast (contrast/contrast) ------------------------------------------

func ratioOfTones(a, b float64) float64 {
	a = clampDouble(0.0, 100.0, a)
	b = clampDouble(0.0, 100.0, b)
	return ratioOfYs(yFromLstar(a), yFromLstar(b))
}

func ratioOfYs(y1, y2 float64) float64 {
	lighter := y2
	if y1 > y2 {
		lighter = y1
	}
	darker := y2
	if lighter == y2 {
		darker = y1
	}
	return (lighter + 5.0) / (darker + 5.0)
}

func lighterTone(tone, ratio float64) float64 {
	if tone < 0.0 || tone > 100.0 {
		return -1.0
	}
	darkY := yFromLstar(tone)
	lightY := float64(ratio*(darkY+5.0)) - 5.0
	got := ratioOfYs(lightY, darkY)
	if got < ratio && math.Abs(got-ratio) > 0.04 {
		return -1
	}
	v := lstarFromY(lightY) + 0.4
	if v < 0 || v > 100 {
		return -1
	}
	return v
}

func darkerTone(tone, ratio float64) float64 {
	if tone < 0.0 || tone > 100.0 {
		return -1.0
	}
	lightY := yFromLstar(tone)
	darkY := ((lightY + 5.0) / ratio) - 5.0
	got := ratioOfYs(lightY, darkY)
	if got < ratio && math.Abs(got-ratio) > 0.04 {
		return -1
	}
	v := lstarFromY(darkY) - 0.4
	if v < 0 || v > 100 {
		return -1
	}
	return v
}

func lighterUnsafe(tone, ratio float64) float64 {
	if v := lighterTone(tone, ratio); v >= 0.0 {
		return v
	}
	return 100.0
}

func darkerUnsafe(tone, ratio float64) float64 {
	if v := darkerTone(tone, ratio); v >= 0.0 {
		return v
	}
	return 0.0
}

// ---- dislike (dislike/dislike_analyzer) ------------------------------------

// fixIfDisliked lifts the dark yellow-greens people read as bile.
func fixIfDisliked(h HCT) HCT {
	hue := jsRound(h.Hue)
	if hue >= 90.0 && hue <= 111.0 && jsRound(h.Chroma) > 16.0 && jsRound(h.Tone) < 65.0 {
		return NewHCT(h.Hue, h.Chroma, 70.0)
	}
	return h
}

// ---- temperature (temperature/temperature_cache), analogous colours only ---

// analogous is TemperatureCache.analogous: count colours spread over the
// colour wheel by perceived temperature, the input in the middle. The content
// variant takes its tertiary from here.
func analogous(input HCT, count, divisions int) []HCT {
	// Every hue at the input's chroma and tone, then the input itself - the
	// original keys its temperatures by object, so the input is the 362nd.
	all := make([]HCT, 0, 362)
	for hue := 0.0; hue <= 360.0; hue += 1.0 {
		all = append(all, NewHCT(hue, input.Chroma, input.Tone))
	}
	byHue := all[:361]
	all = append(all, input)
	temps := make([]float64, len(all))
	for i, h := range all {
		temps[i] = rawTemperature(h)
	}
	order := make([]int, len(all))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return temps[order[a]] < temps[order[b]] })
	coldest, warmest := temps[order[0]], temps[order[len(order)-1]]
	relative := func(i int) float64 {
		r := warmest - coldest
		if r == 0.0 {
			return 0.5
		}
		return (temps[i] - coldest) / r
	}

	startHue := int(jsRound(input.Hue))
	lastTemp := relative(startHue)
	colors := []HCT{byHue[startHue]}
	total := 0.0
	for i := range 360 {
		temp := relative(sanitizeDegreesInt(startHue + i))
		total += math.Abs(temp - lastTemp)
		lastTemp = temp
	}
	hueAddend := 1
	step := total / float64(divisions)
	sum := 0.0
	lastTemp = relative(startHue)
	for len(colors) < divisions {
		idx := sanitizeDegreesInt(startHue + hueAddend)
		h := byHue[idx]
		temp := relative(idx)
		sum += math.Abs(temp - lastTemp)
		satisfied := sum >= float64(float64(len(colors))*step)
		addend := 1
		for satisfied && len(colors) < divisions {
			colors = append(colors, h)
			satisfied = sum >= float64(float64(len(colors)+addend)*step)
			addend++
		}
		lastTemp = temp
		hueAddend++
		if hueAddend > 360 {
			for len(colors) < divisions {
				colors = append(colors, h)
			}
			break
		}
	}

	answers := []HCT{input}
	inc := int(math.Floor(float64(count-1) / 2.0))
	for i := 1; i < inc+1; i++ {
		index := 0 - i
		for index < 0 {
			index += len(colors)
		}
		if index >= len(colors) {
			index %= len(colors)
		}
		answers = append([]HCT{colors[index]}, answers...)
	}
	dec := count - inc - 1
	for i := 1; i < dec+1; i++ {
		index := i
		if index >= len(colors) {
			index %= len(colors)
		}
		answers = append(answers, colors[index])
	}
	return answers
}

func rawTemperature(h HCT) float64 {
	l, a, b := labFromARGB(h.ARGB())
	_ = l
	hue := sanitizeDegreesDouble(float64(math.Atan2(b, a)*180.0) / math.Pi)
	chroma := math.Sqrt((float64(a * a)) + (float64(b * b)))
	return -0.5 + float64(float64(0.02*math.Pow(chroma, 1.07))*math.Cos(float64(sanitizeDegreesDouble(hue-50.0)*math.Pi)/180.0))
}

func labFromARGB(c ARGB) (float64, float64, float64) {
	lr, lg, lb := linearized(c.red()), linearized(c.green()), linearized(c.blue())
	m := srgbToXYZ
	x := float64(m[0][0]*lr) + float64(m[0][1]*lg) + float64(m[0][2]*lb)
	y := float64(m[1][0]*lr) + float64(m[1][1]*lg) + float64(m[1][2]*lb)
	z := float64(m[2][0]*lr) + float64(m[2][1]*lg) + float64(m[2][2]*lb)
	fx := labF(x / whitePointD65[0])
	fy := labF(y / whitePointD65[1])
	fz := labF(z / whitePointD65[2])
	return float64(116.0*fy) - 16, float64(500.0 * (fx - fy)), float64(200.0 * (fy - fz))
}
