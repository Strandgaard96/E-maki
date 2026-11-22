// Package panel prepares frames for a Spectra 6 e-ink panel and serves them
// using Fugleramme's panel protocol.
//
// The palette, tone mapping (pop) and .e6 packing are ported from Fugleramme
// (https://github.com/arnegiacomo/fugleramme, src/fugleramme/render/dither.py
// and src/fugleramme/e6.py), Copyright (c) 2026 Arne Giacomo Munthe-Kaas,
// MIT License. See third_party/fugleramme/LICENSE.
package panel

import (
	"fmt"
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

// Palette is the Spectra 6 palette in driver index order, the default inks.
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

// Dither reduces an image to the panel's inks with Atkinson error
// diffusion: each pixel takes the nearest ink and passes 1/8 of its error to
// six neighbours, so 1/4 is dropped. That keeps flat areas clean and contrast
// high. Follows charnley/eink-art-gallery's atkinson_dither_rgb. The result
// is meant to go to the panel as is; its driver must not dither it again.
// Any palette works: nil is Palette, and a black-and-white or 4-grey panel
// passes its own (see Inks).
func Dither(src image.Image, inks color.Palette) *image.Paletted {
	if len(inks) == 0 {
		inks = Palette
	}
	w, h, px := pop(src)
	out := image.NewPaletted(image.Rect(0, 0, w, h), inks)
	buf := make([][3]int, w*h)
	for i, c := range px {
		buf[i] = [3]int{int(c[0]), int(c[1]), int(c[2])}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := buf[y*w+x]
			idx := nearest(inks, c)
			out.Pix[y*out.Stride+x] = uint8(idx)
			p := inks[idx].(color.RGBA)
			pc := [3]int{int(p.R), int(p.G), int(p.B)}
			var e [3]int
			for k := range e {
				e[k] = floorDiv8(c[k] - pc[k])
			}
			for _, d := range atkinson {
				nx, ny := x+d[0], y+d[1]
				if nx < 0 || nx >= w || ny >= h {
					continue
				}
				n := &buf[ny*w+nx]
				for k := range e {
					n[k] = clip8(n[k] + e[k])
				}
			}
		}
	}
	return out
}

// atkinson lists the neighbours (dx, dy) that each get 1/8 of the error.
var atkinson = [6][2]int{{1, 0}, {2, 0}, {-1, 1}, {0, 1}, {1, 1}, {0, 2}}

// floorDiv8 rounds toward minus infinity, like Python's // in the reference.
func floorDiv8(v int) int { return v >> 3 }

func clip8(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

// nearest finds the ink closest in RGB.
func nearest(inks color.Palette, c [3]int) int {
	best, bestD := 0, math.MaxInt
	for i, pc := range inks {
		p := pc.(color.RGBA)
		dr, dg, db := c[0]-int(p.R), c[1]-int(p.G), c[2]-int(p.B)
		d := dr*dr + dg*dg + db*db
		if d < bestD {
			best, bestD = i, d
		}
	}
	return best
}

// Inks parses a panel's inks, given as "#rrggbb" in its driver's index
// order. None is Palette.
func Inks(hex []string) (color.Palette, error) {
	if len(hex) == 0 {
		return Palette, nil
	}
	if len(hex) < 2 || len(hex) > 16 {
		return nil, fmt.Errorf("a panel has 2 to 16 inks, not %d", len(hex))
	}
	p := make(color.Palette, len(hex))
	for i, h := range hex {
		var r, g, b uint8
		if n, err := fmt.Sscanf(h, "#%02x%02x%02x", &r, &g, &b); n != 3 || err != nil || len(h) != 7 {
			return nil, fmt.Errorf("ink %q is not #rrggbb", h)
		}
		p[i] = color.RGBA{r, g, b, 255}
	}
	return p, nil
}

func sq(v float64) float64 { return v * v }

func clamp(v float64) float64 { return math.Max(0, math.Min(255, v)) }
