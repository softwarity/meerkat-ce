// Package m3color generates Material 3 colour schemes the way Google's Material
// Theme Builder does, so a theme exported from
// https://material-foundation.github.io/material-theme-builder/ and one made in
// Meerkat from the same six colours are the same theme, role for role.
//
// It is a Go port of the parts of material-color-utilities
// (https://github.com/material-foundation/material-color-utilities, TypeScript
// release 0.3.0, Copyright 2021-2023 Google LLC, Apache License 2.0) that a
// scheme needs: the HCT colour space and its solver, tonal palettes, the 2021
// dynamic-colour spec with its contrast levels, and the tonal-spot and content
// variants. The console generates the same schemes with that very release, for
// the live preview; golden files written by it hold the two to the same output
// (see m3color_test.go), because a preview that drew one colour and a save that
// stored another would be worse than no preview.
//
// The arithmetic is kept in the order the original writes it, and every
// product is wrapped in an explicit float64() - the one thing the Go spec says
// stops the compiler fusing a multiply and an add into an FMA, which it does
// on arm64 and not on amd64. Results feed thresholds (a contrast ratio of 4.5,
// a tone that must not drop below 0), and pure black came out a tone of
// -2.2e-16 on a Mac and 0 on a server: one colour, two schemes. Write a new
// product the same way.
package m3color

import "math"

// ---- colour utilities (utils/color_utils, utils/math_utils) ----------------

var srgbToXYZ = [3][3]float64{
	{0.41233895, 0.35762064, 0.18051042},
	{0.2126, 0.7152, 0.0722},
	{0.01932141, 0.11916382, 0.95034478},
}

var whitePointD65 = [3]float64{95.047, 100.0, 108.883}

// ARGB is a colour as 0xAARRGGBB, the representation the original works in.
type ARGB uint32

func argbFromRGB(r, g, b int) ARGB {
	return ARGB(0xff000000 | uint32(r&255)<<16 | uint32(g&255)<<8 | uint32(b&255))
}

func (c ARGB) red() int   { return int(c>>16) & 255 }
func (c ARGB) green() int { return int(c>>8) & 255 }
func (c ARGB) blue() int  { return int(c) & 255 }

func argbFromLinrgb(l [3]float64) ARGB {
	return argbFromRGB(delinearized(l[0]), delinearized(l[1]), delinearized(l[2]))
}

func xyzFromARGB(c ARGB) [3]float64 {
	return matrixMultiply([3]float64{linearized(c.red()), linearized(c.green()), linearized(c.blue())}, srgbToXYZ)
}

func argbFromLstar(lstar float64) ARGB {
	comp := delinearized(yFromLstar(lstar))
	return argbFromRGB(comp, comp, comp)
}

func lstarFromARGB(c ARGB) float64 {
	y := xyzFromARGB(c)[1]
	return float64(116.0*labF(y/100.0)) - 16.0
}

func yFromLstar(lstar float64) float64 { return float64(100.0 * labInvf((lstar+16.0)/116.0)) }

func lstarFromY(y float64) float64 { return float64(labF(y/100.0)*116.0) - 16.0 }

func linearized(component int) float64 {
	n := float64(component) / 255.0
	if n <= 0.040449936 {
		return float64(n / 12.92 * 100.0)
	}
	return float64(math.Pow((n+0.055)/1.055, 2.4) * 100.0)
}

func delinearized(component float64) int {
	n := component / 100.0
	var d float64
	if n <= 0.0031308 {
		d = float64(n * 12.92)
	} else {
		d = float64(1.055*math.Pow(n, 1.0/2.4)) - 0.055
	}
	return clampInt(0, 255, int(jsRound(float64(d*255.0))))
}

func labF(t float64) float64 {
	const e = 216.0 / 24389.0
	const kappa = 24389.0 / 27.0
	if t > e {
		return math.Pow(t, 1.0/3.0)
	}
	return (float64(kappa*t) + 16) / 116
}

func labInvf(ft float64) float64 {
	const e = 216.0 / 24389.0
	const kappa = 24389.0 / 27.0
	ft3 := float64(float64(ft*ft) * ft)
	if ft3 > e {
		return ft3
	}
	return (float64(116*ft) - 16) / kappa
}

// jsRound is JavaScript's Math.round: halves go UP, also below zero, where Go's
// math.Round goes away from zero.
func jsRound(x float64) float64 { return math.Floor(x + 0.5) }

func signum(x float64) float64 {
	switch {
	case x < 0:
		return -1
	case x == 0:
		return 0
	}
	return 1
}

func lerp(start, stop, amount float64) float64 {
	return float64((1.0-amount)*start) + float64(amount*stop)
}

func clampInt(lo, hi, v int) int { return min(hi, max(lo, v)) }

func clampDouble(lo, hi, v float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func sanitizeDegreesInt(d int) int {
	d %= 360
	if d < 0 {
		d += 360
	}
	return d
}

func sanitizeDegreesDouble(d float64) float64 {
	d = math.Mod(d, 360.0)
	if d < 0 {
		d += 360.0
	}
	return d
}

func matrixMultiply(row [3]float64, m [3][3]float64) [3]float64 {
	return [3]float64{float64(row[0]*m[0][0]) + float64(row[1]*m[0][1]) + float64(row[2]*m[0][2]), float64(row[0]*m[1][0]) + float64(row[1]*m[1][1]) + float64(row[2]*m[1][2]), float64(row[0]*m[2][0]) + float64(row[1]*m[2][1]) + float64(row[2]*m[2][2])}
}

// ---- viewing conditions (hct/viewing_conditions) --------------------------

type viewingConditions struct {
	n, aw, nbb, ncb, c, nc float64
	rgbD                   [3]float64
	fl, fLRoot, z          float64
}

// defaultViewing is ViewingConditions.DEFAULT: sRGB white, an adapting
// luminance of 200/pi * Y(50)/100, a mid-grey background, average surround.
var defaultViewing = makeViewingConditions()

func makeViewingConditions() viewingConditions {
	xyz := whitePointD65
	adapting := float64((200.0/math.Pi)*yFromLstar(50.0)) / 100.0
	const backgroundLstar, surround = 50.0, 2.0
	rW := float64(xyz[0]*0.401288) + float64(xyz[1]*0.650173) + float64(xyz[2]*-0.051461)
	gW := float64(xyz[0]*-0.250268) + float64(xyz[1]*1.204414) + float64(xyz[2]*0.045854)
	bW := float64(xyz[0]*-0.002079) + float64(xyz[1]*0.048952) + float64(xyz[2]*0.953127)
	f := 0.8 + surround/10.0
	var c float64
	if f >= 0.9 {
		c = lerp(0.59, 0.69, float64((f-0.9)*10.0))
	} else {
		c = lerp(0.525, 0.59, float64((f-0.8)*10.0))
	}
	d := float64(f * (1.0 - float64((1.0/3.6)*math.Exp((-adapting-42.0)/92.0))))
	d = clampDouble(0, 1, d)
	nc := f
	rgbD := [3]float64{float64(d*(100.0/rW)) + 1.0 - d, float64(d*(100.0/gW)) + 1.0 - d, float64(d*(100.0/bW)) + 1.0 - d}
	k := 1.0 / (float64(5.0*adapting) + 1.0)
	k4 := float64(float64(float64(k*k)*k) * k)
	k4F := 1.0 - k4
	fl := float64(k4*adapting) + float64(float64(float64(0.1*k4F)*k4F)*math.Cbrt(float64(5.0*adapting)))
	n := yFromLstar(backgroundLstar) / whitePointD65[1]
	z := 1.48 + math.Sqrt(n)
	nbb := 0.725 / math.Pow(n, 0.2)
	ncb := nbb
	af := [3]float64{
		math.Pow((float64(float64(fl*rgbD[0])*rW))/100.0, 0.42),
		math.Pow((float64(float64(fl*rgbD[1])*gW))/100.0, 0.42),
		math.Pow((float64(float64(fl*rgbD[2])*bW))/100.0, 0.42),
	}
	a := [3]float64{
		(float64(400.0 * af[0])) / (af[0] + 27.13),
		(float64(400.0 * af[1])) / (af[1] + 27.13),
		(float64(400.0 * af[2])) / (af[2] + 27.13),
	}
	aw := float64((float64(2.0*a[0]) + a[1] + float64(0.05*a[2])) * nbb)
	return viewingConditions{n: n, aw: aw, nbb: nbb, ncb: ncb, c: c, nc: nc, rgbD: rgbD, fl: fl, fLRoot: math.Pow(fl, 0.25), z: z}
}

// ---- CAM16, only what HCT reads from it (hct/cam16) ------------------------

// cam16HueChroma is Cam16.fromInt reduced to the hue and chroma an HCT keeps.
func cam16HueChroma(argb ARGB) (hue, chroma float64) {
	vc := defaultViewing
	redL, greenL, blueL := linearized(argb.red()), linearized(argb.green()), linearized(argb.blue())
	x := float64(0.41233895*redL) + float64(0.35762064*greenL) + float64(0.18051042*blueL)
	y := float64(0.2126*redL) + float64(0.7152*greenL) + float64(0.0722*blueL)
	z := float64(0.01932141*redL) + float64(0.11916382*greenL) + float64(0.95034478*blueL)
	rC := float64(0.401288*x) + float64(0.650173*y) - float64(0.051461*z)
	gC := float64(-0.250268*x) + float64(1.204414*y) + float64(0.045854*z)
	bC := float64(-0.002079*x) + float64(0.048952*y) + float64(0.953127*z)
	rD := float64(vc.rgbD[0] * rC)
	gD := float64(vc.rgbD[1] * gC)
	bD := float64(vc.rgbD[2] * bC)
	rAF := math.Pow((float64(vc.fl*math.Abs(rD)))/100.0, 0.42)
	gAF := math.Pow((float64(vc.fl*math.Abs(gD)))/100.0, 0.42)
	bAF := math.Pow((float64(vc.fl*math.Abs(bD)))/100.0, 0.42)
	rA := (float64(float64(signum(rD)*400.0) * rAF)) / (rAF + 27.13)
	gA := (float64(float64(signum(gD)*400.0) * gAF)) / (gAF + 27.13)
	bA := (float64(float64(signum(bD)*400.0) * bAF)) / (bAF + 27.13)
	a := (float64(11.0*rA) + float64(-12.0*gA) + bA) / 11.0
	b := (rA + gA - float64(2.0*bA)) / 9.0
	u := (float64(20.0*rA) + float64(20.0*gA) + float64(21.0*bA)) / 20.0
	p2 := (float64(40.0*rA) + float64(20.0*gA) + bA) / 20.0
	atanDegrees := (float64(math.Atan2(b, a) * 180.0)) / math.Pi
	switch {
	case atanDegrees < 0:
		hue = atanDegrees + 360.0
	case atanDegrees >= 360:
		hue = atanDegrees - 360.0
	default:
		hue = atanDegrees
	}
	ac := float64(p2 * vc.nbb)
	j := float64(100.0 * math.Pow(ac/vc.aw, float64(vc.c*vc.z)))
	huePrime := hue
	if hue < 20.14 {
		huePrime = hue + 360
	}
	eHue := float64(0.25 * (math.Cos((float64(huePrime*math.Pi))/180.0+2.0) + 3.8))
	p1 := float64(float64(float64((50000.0/13.0)*eHue)*vc.nc) * vc.ncb)
	t := (float64(p1 * math.Sqrt(float64(a*a)+float64(b*b)))) / (u + 0.305)
	alpha := float64(math.Pow(t, 0.9) * math.Pow(1.64-math.Pow(0.29, vc.n), 0.73))
	chroma = float64(alpha * math.Sqrt(j/100.0))
	return hue, chroma
}

// ---- HCT (hct/hct) ---------------------------------------------------------

// HCT is a colour in hue, chroma and tone: the space every decision below is
// made in, because tone is perceptual lightness (L*) and a contrast ratio is a
// function of tone alone.
type HCT struct {
	Hue, Chroma, Tone float64
	argb              ARGB
}

// FromARGB measures a colour.
func FromARGB(c ARGB) HCT {
	h, ch := cam16HueChroma(c)
	return HCT{Hue: h, Chroma: ch, Tone: lstarFromARGB(c), argb: c}
}

// NewHCT finds the sRGB colour closest to the asked hue, chroma and tone - the
// chroma is a wish, the gamut decides how much of it there is.
func NewHCT(hue, chroma, tone float64) HCT { return FromARGB(solveToInt(hue, chroma, tone)) }

// ARGB is the colour itself.
func (h HCT) ARGB() ARGB { return h.argb }
