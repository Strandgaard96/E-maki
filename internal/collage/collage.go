// Package collage lays the remembered topics out as one page for the panel,
// each with a museum-label caption. By default the first topic is a big hero
// tile and the rest are smaller tiles, on the smallest grid that holds them
// all (up to 4x3); Look picks other arrangements, spacing, background and
// tile shapes.
package collage

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"math/rand/v2"
	"slices"
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
)

// Look is how the page is drawn. The zero Look is the default: hero
// arrangement, normal spacing, paper background, rectangles.
type Look struct {
	// Arrange: "hero" (the first tile biggest, on a grid), "even" (all
	// tiles about the same size, in rows) or "mosaic" (mixed sizes, from
	// Seed). Empty is "hero".
	Arrange string
	// Spacing between tiles and at the page's edge: "none", "thin",
	// "normal" or "wide". Empty is "normal".
	Spacing string
	// Background: "paper", "white" or "black", all inks the panel shows
	// flat. Empty is "paper".
	Background string
	// Shape of each picture: "rectangle", "rounded", "circle" or "arch"
	// (a round top). Empty is "rectangle".
	Shape string
	// Seed picks the mosaic. The same seed gives the same mosaic.
	Seed uint64
}

// Choices for each of Look's fields, the default first.
var (
	Arrangements = []string{"hero", "even", "mosaic"}
	Spacings     = []string{"normal", "none", "thin", "wide"}
	Backgrounds  = []string{"paper", "white", "black"}
	Shapes       = []string{"rectangle", "rounded", "circle", "arch"}
)

// space is the margin at the page's edge and the gutter between tiles, for
// a page whose short side is ref.
func (l Look) space() (margin, gutter float64) {
	switch l.Spacing {
	case "none":
		return 0, 0
	case "thin":
		return 20, 10
	case "wide":
		return 80, 48
	}
	return 44, 26
}

// colours are the page's background and the captions' ink on it.
func (l Look) colours() (bg, fg color.RGBA) {
	switch l.Background {
	case "white":
		return white, ink
	case "black":
		return ink, white
	}
	return paper, ink
}

// DefaultSize is the page when none is given: the 13.3" panel, landscape.
var DefaultSize = image.Pt(1600, 1200)

// scale is a page's sizes relative to ref.
func scale(size image.Point) float64 { return float64(min(size.X, size.Y)) / ref }

// Tile is one topic on the page.
type Tile struct {
	Title string // the topic, shown as the caption
	Meta  string // small line under it, e.g. "KITCHEN · SUN · ×7"; no Title and Meta means no caption
	Image image.Image
	// A sample tile has no Image: it is flat Ink with a Disc in the middle,
	// drawn at the tile's own size so the disc is never cut off.
	Ink, Disc color.RGBA
}

var (
	paper = color.RGBA{233, 225, 202, 255} // Fugleramme's PANEL_PAPER, which the dither keeps flat
	ink   = color.RGBA{24, 22, 22, 255}
	white = color.RGBA{252, 250, 244, 255}
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
	{4, 2, []cell{{0, 0, 2, 2}, {2, 0, 1, 1}, {3, 0, 1, 1}, {2, 1, 1, 1}, {3, 1, 1, 1}}},
	{3, 3, []cell{{0, 0, 2, 2}, {2, 0, 1, 1}, {2, 1, 1, 1}, {0, 2, 1, 1}, {1, 2, 1, 1}, {2, 2, 1, 1}}},
	// 7 and 8 are the 9 layout with bottom tiles joined in pairs.
	{4, 3, []cell{
		{0, 0, 2, 2}, {0, 2, 2, 1}, {2, 2, 2, 1},
		{2, 0, 1, 1}, {3, 0, 1, 1}, {2, 1, 1, 1}, {3, 1, 1, 1},
	}},
	{4, 3, []cell{
		{0, 0, 2, 2}, {0, 2, 2, 1},
		{2, 0, 1, 1}, {3, 0, 1, 1}, {2, 1, 1, 1}, {3, 1, 1, 1}, {2, 2, 1, 1}, {3, 2, 1, 1},
	}},
	{4, 3, []cell{
		{0, 0, 2, 2}, {2, 0, 1, 1}, {3, 0, 1, 1}, {2, 1, 1, 1}, {3, 1, 1, 1},
		{0, 2, 1, 1}, {1, 2, 1, 1}, {2, 2, 1, 1}, {3, 2, 1, 1},
	}},
}

// MaxTiles is how many topics fit on a page.
var MaxTiles = len(layouts[len(layouts)-1].cells)

// Rects is where n tiles go on a page of the given size, caption included,
// in tile order: the biggest first. It holds at most MaxTiles rectangles.
func Rects(n int, size image.Point, look Look) []image.Rectangle {
	n = min(n, MaxTiles)
	if n <= 0 {
		return nil
	}
	m, g := look.space()
	k := scale(size)
	m, g = m*k, g*k
	inner := rect{m, m, float64(size.X) - m, float64(size.Y) - m}
	var rs []rect
	switch look.Arrange {
	case "even":
		rs = even(n, inner, g)
	case "mosaic":
		rs = mosaic(n, inner, g, rand.New(rand.NewPCG(look.Seed, uint64(n))))
	default:
		rs = hero(n, inner, g, size.Y > size.X)
	}
	out := make([]image.Rectangle, len(rs))
	for i, r := range rs {
		out[i] = image.Rect(int(r.x0), int(r.y0), int(r.x1), int(r.y1))
	}
	if look.Arrange == "even" || look.Arrange == "mosaic" {
		slices.SortStableFunc(out, func(a, b image.Rectangle) int { return b.Dx()*b.Dy() - a.Dx()*a.Dy() })
	}
	return out
}

type rect struct{ x0, y0, x1, y1 float64 }

func (r rect) w() float64 { return r.x1 - r.x0 }
func (r rect) h() float64 { return r.y1 - r.y0 }

// hero places n tiles on the smallest grid layout that holds them. On a
// portrait page the layouts are turned on their side.
func hero(n int, in rect, g float64, portrait bool) []rect {
	l := layouts[len(layouts)-1]
	for _, c := range layouts {
		if len(c.cells) >= n {
			l = c
			break
		}
	}
	if portrait {
		t := layout{l.rows, l.cols, nil}
		for _, c := range l.cells {
			t.cells = append(t.cells, cell{c.r, c.c, c.rs, c.cs})
		}
		l = t
	}
	cw := (in.w() - float64(l.cols-1)*g) / float64(l.cols)
	ch := (in.h() - float64(l.rows-1)*g) / float64(l.rows)
	var out []rect
	for i := 0; i < n && i < len(l.cells); i++ {
		c := l.cells[i]
		x0 := in.x0 + float64(c.c)*(cw+g)
		y0 := in.y0 + float64(c.r)*(ch+g)
		out = append(out, rect{x0, y0, x0 + float64(c.cs)*cw + float64(c.cs-1)*g, y0 + float64(c.rs)*ch + float64(c.rs-1)*g})
	}
	return out
}

// even places n tiles in rows of equal height, each row filled edge to
// edge, so there are no holes. It picks the number of rows whose tiles come
// closest to the pictures' 4:3.
func even(n int, in rect, g float64) []rect {
	rowsFor := func(r int) []int {
		counts := make([]int, r)
		for i := range counts {
			counts[i] = n / r
			if i >= r-n%r {
				counts[i]++ // the fuller rows at the bottom
			}
		}
		return counts
	}
	best, worst := 1, math.Inf(1)
	for r := 1; r <= n; r++ {
		h := (in.h() - float64(r-1)*g) / float64(r)
		bad := 0.0
		for _, c := range rowsFor(r) {
			w := (in.w() - float64(c-1)*g) / float64(c)
			bad = max(bad, math.Abs(math.Log(w/h/(4.0/3))))
		}
		if bad < worst {
			best, worst = r, bad
		}
	}
	var out []rect
	h := (in.h() - float64(best-1)*g) / float64(best)
	for i, c := range rowsFor(best) {
		w := (in.w() - float64(c-1)*g) / float64(c)
		y0 := in.y0 + float64(i)*(h+g)
		for j := range c {
			x0 := in.x0 + float64(j)*(w+g)
			out = append(out, rect{x0, y0, x0 + w, y0 + h})
		}
	}
	return out
}

// mosaic cuts the page in two again and again, mostly across its longer
// side, at a point rnd moves a little off the even split.
func mosaic(n int, in rect, g float64, rnd *rand.Rand) []rect {
	var cut func(r rect, n int) []rect
	cut = func(r rect, n int) []rect {
		if n == 1 {
			return []rect{r}
		}
		a := n / 2
		if n%2 == 1 && rnd.IntN(2) == 0 {
			a++
		}
		f := float64(a) / float64(n) * (0.8 + 0.4*rnd.Float64())
		f = min(max(f, 0.35), 0.65)
		var p, q rect
		// Pictures are 4:3 and captions take height, so cut wide pieces
		// side by side sooner than square ones.
		if r.w() >= 1.2*r.h() {
			x := r.x0 + (r.w()-g)*f
			p, q = rect{r.x0, r.y0, x, r.y1}, rect{x + g, r.y0, r.x1, r.y1}
		} else {
			y := r.y0 + (r.h()-g)*f
			p, q = rect{r.x0, r.y0, r.x1, y}, rect{r.x0, y + g, r.x1, r.y1}
		}
		if rnd.IntN(2) == 0 { // the bigger share on either side
			p, q = q, p
			a = n - a
		}
		return append(cut(p, a), cut(q, n-a)...)
	}
	return cut(in, n)
}

// Render draws up to MaxTiles tiles on a page of the given size (as the
// viewer sees it), the first one biggest. Captions are pure black or white:
// grey text breaks up once dithered to the panel's inks.
func Render(tiles []Tile, size image.Point, look Look) *image.RGBA {
	page := image.NewRGBA(image.Rectangle{Max: size})
	if len(tiles) == 0 {
		draw.Draw(page, page.Bounds(), &image.Uniform{paper}, image.Point{}, draw.Src)
		empty(page)
		return page
	}
	bg, fg := look.colours()
	draw.Draw(page, page.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	k := scale(size)
	for i, r := range Rects(len(tiles), size, look) {
		t := tiles[i]
		x0, y0, x1, y1 := r.Min.X, r.Min.Y, r.Max.X, r.Max.Y
		titleSize, metaSize, labelH := 28.0, 20.0, 80.0
		switch w := float64(x1-x0) / k; {
		case w >= 1200:
			titleSize, metaSize, labelH = 52, 32, 132
		case w >= 600:
			titleSize, metaSize, labelH = 40, 26, 104
		}
		titleSize, metaSize, labelH = titleSize*k, metaSize*k, labelH*k
		// Small panels: captions stay readable, the picture gives way.
		titleSize, metaSize = max(titleSize, minText*4/3), max(metaSize, minText)
		labelH = max(labelH, titleSize+metaSize+20*k+4)
		if t.Title == "" && t.Meta == "" {
			labelH = 0
		}
		art := image.Rect(x0, y0, x1, y1-int(labelH))
		if look.Shape == "circle" {
			art = square(art)
		}
		switch {
		case t.Image != nil:
			cover(page, art, t.Image)
		case t.Ink == bg: // a sample would vanish into the background
			sample(page, art, t.Disc, t.Ink)
		case t.Ink.A != 0:
			sample(page, art, t.Ink, t.Disc)
		}
		mask(page, art, look.Shape, bg)
		if labelH > 0 {
			// Under a circle the caption is centred, like the circle.
			at := func(s string, size float64) int {
				if look.Shape != "circle" {
					return x0
				}
				return x0 + (x1-x0-textWidth(s, size))/2
			}
			title, size := fit(t.Title, titleSize, x1-x0)
			text(page, at(title, size), y1-int(labelH)+int(titleSize+12*k), size, fg, title)
			meta, size := fit(t.Meta, metaSize, x1-x0)
			text(page, at(meta, size), y1-int(4*k), size, fg, meta)
		}
	}
	return page
}

// square is the biggest square in the middle of r.
func square(r image.Rectangle) image.Rectangle {
	s := min(r.Dx(), r.Dy())
	x, y := r.Min.X+(r.Dx()-s)/2, r.Min.Y+(r.Dy()-s)/2
	return image.Rect(x, y, x+s, y+s)
}

// sample fills r with a flat ink and a disc in the middle that fits r.
func sample(page *image.RGBA, r image.Rectangle, ink, disc color.RGBA) {
	draw.Draw(page, r, &image.Uniform{ink}, image.Point{}, draw.Src)
	cx, cy := float64(r.Min.X+r.Max.X)/2, float64(r.Min.Y+r.Max.Y)/2
	rad := 0.3 * float64(min(r.Dx(), r.Dy()))
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy; dx*dx+dy*dy < rad*rad {
				page.SetRGBA(x, y, disc)
			}
		}
	}
}

// mask paints the background over the parts of r outside the shape. The
// edge is hard, with no blending, so the dither keeps it clean.
func mask(page *image.RGBA, r image.Rectangle, shape string, bg color.RGBA) {
	s := float64(min(r.Dx(), r.Dy()))
	var rad float64
	bottom := true // round the bottom corners too
	switch shape {
	case "rounded":
		rad = max(0.06*s, 2)
	case "circle":
		rad = s / 2
	case "arch":
		rad, bottom = min(float64(r.Dx())/2, float64(r.Dy())), false
	default:
		return
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			// the centre of the corner's circle nearest this pixel
			cx := min(max(px, float64(r.Min.X)+rad), float64(r.Max.X)-rad)
			cy := max(py, float64(r.Min.Y)+rad)
			if bottom {
				cy = min(cy, float64(r.Max.Y)-rad)
			}
			if dx, dy := px-cx, py-cy; dx*dx+dy*dy > rad*rad {
				page.SetRGBA(x, y, bg)
			}
		}
	}
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

// Fit scales a picture down to the most any layout draws of it: a page of
// size, in either orientation. So a large picture from the provider does
// not sit in memory at full size. A picture that is small enough comes
// back as it is.
func Fit(src image.Image, size image.Point) image.Image {
	sb := src.Bounds()
	side := float64(max(size.X, size.Y))
	scale := max(side/float64(sb.Dx()), side/float64(sb.Dy()))
	if scale >= 1 {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, int(math.Ceil(float64(sb.Dx())*scale)), int(math.Ceil(float64(sb.Dy())*scale))))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, sb, xdraw.Src, nil)
	return dst
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

func text(page *image.RGBA, x, y int, size float64, c color.RGBA, s string) {
	face, err := opentype.NewFace(regular, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return
	}
	defer face.Close()
	d := font.Drawer{Dst: page, Src: &image.Uniform{c}, Face: face, Dot: fixed.P(x, y)}
	d.DrawString(s)
}

// The panel's inks for the empty page's mark, as flat colours the dither keeps.
var marks = []color.RGBA{{24, 22, 22, 255}, {252, 250, 244, 255}, {232, 186, 52, 255}, {196, 62, 50, 255}, {36, 66, 140, 255}, {52, 112, 72, 255}}

// empty draws the page shown before the first picture, or when every tile
// is gone: E-maki's mark, a small framed grid of the six inks, and a line on
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
	centred(page, top+px(760), 64*k, "E-maki is listening")
	centred(page, top+px(830), 30*k, "A picture appears here when a topic comes up.")
}

func centred(page *image.RGBA, y int, size float64, s string) {
	face, err := opentype.NewFace(regular, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return
	}
	defer face.Close()
	w := font.MeasureString(face, s).Round()
	text(page, (page.Bounds().Dx()-w)/2, y, size, ink, s)
}

// Paused marks the page in its top right corner, in the margin, so people
// in the room can see that E-maki is not listening. On the empty page
// (Render with no tiles) it also replaces "E-maki is listening".
func Paused(page *image.RGBA, empty bool) {
	pg := page.Bounds().Size()
	k := scale(pg)
	px := func(v float64) int { return int(v * k) }
	if empty {
		top := pg.Y/2 - px(ref/2)
		draw.Draw(page, image.Rect(0, top+px(690), pg.X, top+px(780)), &image.Uniform{paper}, image.Point{}, draw.Src)
		centred(page, top+px(760), 64*k, "E-maki is paused")
	}
	size := max(20*k, minText)
	padX, h, m := px(14), int(size*1.6), px(44)
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
	text(page, r.Min.X+padX, r.Max.Y-int(size*0.45), size, ink, "NOT LISTENING")
}
