// Package panel prepares frames for a Spectra 6 e-ink panel and serves them
// using Fugleramme's panel protocol.
//
// The dithering and .e6 packing are ported from Fugleramme
// (https://github.com/arnegiacomo/fugleramme, src/fugleramme/render/dither.py
// and src/fugleramme/e6.py), Copyright (c) 2026 Arne Giacomo Munthe-Kaas,
// MIT License. See third_party/fugleramme/LICENSE.
package panel

import (
	"image"
	"image/color"
	"math"
)

// Driver index order: black, white, yellow, red, blue, green. Fugleramme's
// Inky client relies on this order, so do not reorder.
var (
	desaturated = [6][3]float64{{0, 0, 0}, {255, 255, 255}, {255, 255, 0}, {255, 0, 0}, {0, 0, 255}, {0, 255, 0}}
	saturated   = [6][3]float64{{0, 0, 0}, {161, 164, 165}, {208, 190, 71}, {156, 72, 75}, {61, 59, 94}, {58, 91, 70}}
)

// saturation blends the measured panel colours with the driver's pure ones.
const saturation = 0.5

// Palette is the 6-colour panel palette in driver index order.
var Palette = func() color.Palette {
	p := make(color.Palette, 6)
	for i := range p {
		var c [3]uint8
		for k := 0; k < 3; k++ {
			c[k] = uint8(saturated[i][k]*saturation + desaturated[i][k]*(1-saturation))
		}
		p[i] = color.RGBA{c[0], c[1], c[2], 255}
	}
	return p
}()

var (
	paper    = [3]float64{233, 225, 202} // PANEL_PAPER
	lift     = [3]float64{0.887, 0.906, 0.950}
	pivot    = 182.0
	contrast = 1.15
)

const (
	fade      = 4.0
	halo      = 12.0
	haloReach = 32
)

func boost(v, l float64) float64 { return pivot + (v*l-pivot)*contrast }

// pop darkens subjects so pale areas take ink, while paper-coloured areas and
// the halo around them keep their tone (Fugleramme's _pop).
func pop(src image.Image) (w, h int, px [][3]float64) {
	b := src.Bounds()
	w, h = b.Dx(), b.Dy()
	px = make([][3]float64, w*h)
	dist := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, _ := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
			c := [3]float64{float64(r >> 8), float64(g >> 8), float64(bl >> 8)}
			px[y*w+x] = c
			dist[y*w+x] = math.Sqrt(sq(c[0]-paper[0]) + sq(c[1]-paper[1]) + sq(c[2]-paper[2]))
		}
	}
	reached := reach(w, h, dist)
	for i, c := range px {
		keep := math.Exp(-dist[i] / fade)
		if reached[i] {
			keep = 1
		}
		for k := 0; k < 3; k++ {
			v := boost(c[k], lift[k]) + keep*(paper[k]-boost(paper[k], lift[k]))
			px[i][k] = clamp(math.RoundToEven(v))
		}
	}
	return w, h, px
}

// reach grows the paper (dist<=1) through near-paper pixels (dist<=halo) for
// up to haloReach 4-connected steps, like Fugleramme's _reach(...) > 0.
func reach(w, h int, dist []float64) []bool {
	grown := make([]bool, w*h)
	for i, d := range dist {
		grown[i] = d <= 1
	}
	next := make([]bool, w*h)
	for step := 0; step < haloReach; step++ {
		changed := false
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				i := y*w + x
				n := grown[i] ||
					(x > 0 && grown[i-1]) || (x < w-1 && grown[i+1]) ||
					(y > 0 && grown[i-w]) || (y < h-1 && grown[i+w])
				next[i] = grown[i] || (n && dist[i] <= halo)
				changed = changed || next[i] != grown[i]
			}
		}
		grown, next = next, grown
		if !changed {
			break
		}
	}
	return grown
}

// Dither reduces an image to the 6-colour panel palette with Floyd-Steinberg
// error diffusion, using the same integer arithmetic as Pillow's
// quantize(dither=FLOYDSTEINBERG) that Fugleramme relies on. The result is
// meant to go to the panel as is; its driver must not dither it again.
func Dither(src image.Image) *image.Paletted {
	w, h, px := pop(src)
	out := image.NewPaletted(image.Rect(0, 0, w, h), Palette)
	// errs holds the next row's accumulated error (x16), one slot per column
	// plus one, laid out as in Pillow's Convert.c.
	errs := make([][3]int, w+1)
	for y := 0; y < h; y++ {
		var cur, e0, e1 [3]int // error carried right (x16), and the two pending slots
		for x := 0; x < w; x++ {
			var c [3]int
			for k := 0; k < 3; k++ {
				c[k] = clip8(int(px[y*w+x][k]) + truncDiv16(cur[k]+errs[x+1][k]))
			}
			idx := nearest(c)
			out.Pix[y*out.Stride+x] = uint8(idx)
			p := Palette[idx].(color.RGBA)
			pc := [3]int{int(p.R), int(p.G), int(p.B)}
			for k := 0; k < 3; k++ {
				err := c[k] - pc[k]
				errs[x][k] = 3*err + e0[k]
				e0[k] = 5*err + e1[k]
				e1[k] = err
				cur[k] = 7 * err
			}
		}
		errs[w] = e0
	}
	return out
}

func truncDiv16(v int) int { return v / 16 } // Go, like C, truncates toward zero

func clip8(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

// nearest finds the closest palette entry. Like Pillow's palette cache, it
// measures from the corner of the 4x4x4 cache cell the colour falls in.
func nearest(c [3]int) int {
	c = [3]int{c[0] &^ 3, c[1] &^ 3, c[2] &^ 3}
	best, bestD := 0, math.MaxInt
	for i, pc := range Palette {
		p := pc.(color.RGBA)
		dr, dg, db := c[0]-int(p.R), c[1]-int(p.G), c[2]-int(p.B)
		d := dr*dr + dg*dg + db*db
		if d < bestD {
			best, bestD = i, d
		}
	}
	return best
}

func sq(v float64) float64 { return v * v }

func clamp(v float64) float64 { return math.Max(0, math.Min(255, v)) }
