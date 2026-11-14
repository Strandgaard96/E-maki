// Package collage lays the remembered topics out as one page for the panel:
// the first topic as a big hero tile, the rest as smaller tiles, on the
// smallest grid that holds them all (up to 4x3), each with a museum-label
// caption.
package collage

import (
	"image"
	"image/color"
	"image/draw"
	"strings"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// The sizes below are drawn for a page whose short side is ref pixels (the
// 13.3" Spectra 6, 1600x1200). Other pages scale them by their short side.
const (
	ref = 1200.0
	// minText is the smallest caption in pixels, so captions on a small
	// panel stay readable.
	minText = 10.0
	margin  = 44
	gutter  = 26
)

// DefaultSize is the page when none is given: the 13.3" panel, landscape.
var DefaultSize = image.Pt(1600, 1200)

// scale is a page's sizes relative to ref.
func scale(size image.Point) float64 { return float64(min(size.X, size.Y)) / ref }

// Tile is one topic on the page.
type Tile struct {
	Title string // the topic, shown as the caption
	Meta  string // small line under it, e.g. "KITCHEN · SUN · ×7"; no Title and Meta means no caption
	Image image.Image
}

var (
	paper = color.RGBA{233, 225, 202, 255} // Fugleramme's PANEL_PAPER, which the dither keeps flat
	ink   = color.RGBA{24, 22, 22, 255}
)

type cell struct{ c, r, cs, rs int }

// A layout is a grid and its cells in fill order: the biggest cell first.
type layout struct {
	cols, rows int
	cells      []cell
}

// layouts from small to large. Render picks the smallest that holds all
// tiles, so a page with few topics has no empty cells.
var layouts = []layout{
	{1, 1, []cell{{0, 0, 1, 1}}},
	{2, 1, []cell{{0, 0, 1, 1}, {1, 0, 1, 1}}},
	{3, 2, []cell{{0, 0, 2, 2}, {2, 0, 1, 1}, {2, 1, 1, 1}}},
	{2, 2, []cell{{0, 0, 1, 1}, {1, 0, 1, 1}, {0, 1, 1, 1}, {1, 1, 1, 1}}},
	{3, 3, []cell{{0, 0, 2, 2}, {2, 0, 1, 1}, {2, 1, 1, 1}, {0, 2, 1, 1}, {1, 2, 1, 1}, {2, 2, 1, 1}}},
	{4, 3, []cell{
		{0, 0, 2, 2}, {2, 0, 1, 1}, {3, 0, 1, 1}, {2, 1, 1, 1}, {3, 1, 1, 1},
		{0, 2, 1, 1}, {1, 2, 1, 1}, {2, 2, 1, 1}, {3, 2, 1, 1},
	}},
}

// MaxTiles is how many topics fit on a page.
var MaxTiles = len(layouts[len(layouts)-1].cells)

// Rects is where n tiles go on a page of the given size, caption included,
// in tile order. It holds at most MaxTiles rectangles. On a portrait page
// the layouts are turned on their side.
func Rects(n int, size image.Point) []image.Rectangle {
	l := layouts[len(layouts)-1]
	for _, c := range layouts {
		if len(c.cells) >= n {
			l = c
			break
		}
	}
	if size.Y > size.X {
		t := layout{l.rows, l.cols, nil}
		for _, c := range l.cells {
			t.cells = append(t.cells, cell{c.r, c.c, c.rs, c.cs})
		}
		l = t
	}
	k := scale(size)
	m, g := margin*k, gutter*k
	cw := (float64(size.X) - 2*m - float64(l.cols-1)*g) / float64(l.cols)
	ch := (float64(size.Y) - 2*m - float64(l.rows-1)*g) / float64(l.rows)
	var out []image.Rectangle
	for i := 0; i < n && i < len(l.cells); i++ {
		c := l.cells[i]
		x0 := int(m + float64(c.c)*(cw+g))
		y0 := int(m + float64(c.r)*(ch+g))
		x1 := x0 + int(float64(c.cs)*cw+float64(c.cs-1)*g)
		y1 := y0 + int(float64(c.rs)*ch+float64(c.rs-1)*g)
		out = append(out, image.Rect(x0, y0, x1, y1))
	}
	return out
}

// Render draws up to MaxTiles tiles on a page of the given size (as the
// viewer sees it), the first one biggest. Captions are pure black: grey text
// breaks up once dithered to the panel's inks.
func Render(tiles []Tile, size image.Point) *image.RGBA {
	page := image.NewRGBA(image.Rectangle{Max: size})
	draw.Draw(page, page.Bounds(), &image.Uniform{paper}, image.Point{}, draw.Src)
	if len(tiles) == 0 {
		empty(page)
		return page
	}
	k := scale(size)
	for i, r := range Rects(len(tiles), size) {
		t := tiles[i]
		x0, y0, x1, y1 := r.Min.X, r.Min.Y, r.Max.X, r.Max.Y
		titleSize, metaSize, labelH := 26.0, 13.0, 64.0
		switch w := float64(x1-x0) / k; {
		case w >= 1200:
			titleSize, metaSize, labelH = 52, 20, 112
		case w >= 600:
			titleSize, metaSize, labelH = 40, 16, 88
		}
		titleSize, metaSize, labelH = titleSize*k, metaSize*k, labelH*k
		// Small panels: captions stay readable, the picture gives way.
		titleSize, metaSize = max(titleSize, minText*4/3), max(metaSize, minText)
		labelH = max(labelH, titleSize+metaSize+20*k+4)
		if t.Title == "" && t.Meta == "" {
			labelH = 0
		}
		art := image.Rect(x0, y0, x1, y1-int(labelH))
		if t.Image != nil {
			cover(page, art, t.Image)
		}
		if labelH > 0 {
			title, size := fit(t.Title, titleSize, x1-x0)
			text(page, x0, y1-int(labelH)+int(titleSize+12*k), size, title)
			meta, size := fit(t.Meta, metaSize, x1-x0)
			text(page, x0, y1-int(4*k), size, meta)
		}
	}
	return page
}

// cover scales src to fill dst, cropping the overflow from the centre.
func cover(page *image.RGBA, dst image.Rectangle, src image.Image) {
	sb := src.Bounds()
	scale := max(float64(dst.Dx())/float64(sb.Dx()), float64(dst.Dy())/float64(sb.Dy()))
	cw, ch := int(float64(dst.Dx())/scale), int(float64(dst.Dy())/scale)
	crop := image.Rect(sb.Min.X+(sb.Dx()-cw)/2, sb.Min.Y+(sb.Dy()-ch)/2, 0, 0)
	crop.Max = crop.Min.Add(image.Pt(cw, ch))
	xdraw.CatmullRom.Scale(page, dst, src, crop, xdraw.Src, nil)
}

var regular, _ = opentype.Parse(goregular.TTF)

// fit makes a caption fit its tile's width: a smaller size first, down to
// three quarters, then cut at a word with an ellipsis.
func fit(s string, size float64, width int) (string, float64) {
	if textWidth(s, size) <= width {
		return s, size
	}
	size = max(size*float64(width)/float64(textWidth(s, size)), size*0.75)
	words := strings.Fields(s)
	for len(words) > 1 && textWidth(s, size) > width {
		words = words[:len(words)-1]
		s = strings.Join(words, " ") + "…"
	}
	return s, size
}

func textWidth(s string, size float64) int {
	face, err := opentype.NewFace(regular, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return 0
	}
	defer face.Close()
	return font.MeasureString(face, s).Ceil()
}

func text(page *image.RGBA, x, y int, size float64, s string) {
	face, err := opentype.NewFace(regular, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return
	}
	defer face.Close()
	d := font.Drawer{Dst: page, Src: &image.Uniform{ink}, Face: face, Dot: fixed.P(x, y)}
	d.DrawString(s)
}

// The panel's inks for the empty page's mark, as flat colours the dither keeps.
var marks = []color.RGBA{{24, 22, 22, 255}, {252, 250, 244, 255}, {232, 186, 52, 255}, {196, 62, 50, 255}, {36, 66, 140, 255}, {52, 112, 72, 255}}

// empty draws the page shown before the first picture, or when every tile
// is gone: E-kiru's mark, a small framed grid of the six inks, and a line on
// what will appear. Flat shapes and black text, so it stays crisp in six
// colours.
func empty(page *image.RGBA) {
	size := page.Bounds().Size()
	k := scale(size)
	// Drawn for ref and scaled; down from the middle of the page.
	px := func(v float64) int { return int(v * k) }
	top := size.Y/2 - px(ref/2)
	cell, gap, pad, frame := px(70), px(12), px(22), px(18)
	gw, gh := 3*cell+2*gap, 2*cell+gap
	x0, y0 := (size.X-gw)/2, top+px(360)
	fill := func(r image.Rectangle, c color.Color) { draw.Draw(page, r, &image.Uniform{c}, image.Point{}, draw.Src) }
	fill(image.Rect(x0-pad-frame, y0-pad-frame, x0+gw+pad+frame, y0+gh+pad+frame), ink)
	fill(image.Rect(x0-pad, y0-pad, x0+gw+pad, y0+gh+pad), marks[1])
	for i, c := range marks {
		x, y := x0+(i%3)*(cell+gap), y0+(i/3)*(cell+gap)
		if i == 1 { // the white ink needs an edge on white paper
			e := max(px(3), 1)
			fill(image.Rect(x, y, x+cell, y+cell), ink)
			fill(image.Rect(x+e, y+e, x+cell-e, y+cell-e), c)
			continue
		}
		fill(image.Rect(x, y, x+cell, y+cell), c)
	}
	centred(page, top+px(760), 64*k, "E-kiru is listening")
	centred(page, top+px(830), 30*k, "A picture appears here when a topic comes up.")
}

func centred(page *image.RGBA, y int, size float64, s string) {
	face, err := opentype.NewFace(regular, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return
	}
	defer face.Close()
	w := font.MeasureString(face, s).Round()
	text(page, (page.Bounds().Dx()-w)/2, y, size, s)
}

// Paused marks the page in its top right corner, in the margin, so people
// in the room can see that E-kiru is not listening. On the empty page
// (Render with no tiles) it also replaces "E-kiru is listening".
func Paused(page *image.RGBA, empty bool) {
	pg := page.Bounds().Size()
	k := scale(pg)
	px := func(v float64) int { return int(v * k) }
	if empty {
		top := pg.Y/2 - px(ref/2)
		draw.Draw(page, image.Rect(0, top+px(690), pg.X, top+px(780)), &image.Uniform{paper}, image.Point{}, draw.Src)
		centred(page, top+px(760), 64*k, "E-kiru is paused")
	}
	size := max(20*k, minText)
	padX, h, m := px(14), int(size*1.6), px(margin)
	face, err := opentype.NewFace(regular, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return
	}
	w := font.MeasureString(face, "NOT LISTENING").Round()
	face.Close()
	y0 := max((m-h)/2, 0)
	r := image.Rect(pg.X-m-w-2*padX, y0, pg.X-m, y0+h)
	e := max(px(2), 1)
	draw.Draw(page, r, &image.Uniform{ink}, image.Point{}, draw.Src)
	draw.Draw(page, r.Inset(e), &image.Uniform{marks[1]}, image.Point{}, draw.Src)
	text(page, r.Min.X+padX, r.Max.Y-int(size*0.45), size, "NOT LISTENING")
}
