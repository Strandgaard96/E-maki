package collage

import (
	"image"
	"image/color"
	"testing"
)

// Every look places every tile count inside the page, without overlaps,
// the biggest tile first.
func TestLooksPlaceEveryTile(t *testing.T) {
	for _, size := range []image.Point{DefaultSize, {1200, 1600}, {800, 480}} {
		for _, a := range Arrangements {
			for _, sp := range Spacings {
				for seed := range uint64(5) {
					look := Look{Arrange: a, Spacing: sp, Seed: seed}
					for n := 1; n <= MaxTiles; n++ {
						rs := Rects(n, size, look)
						if len(rs) != n {
							t.Fatalf("%v %+v: %d tiles got %d places", size, look, n, len(rs))
						}
						page := image.Rectangle{Max: size}
						for i, r := range rs {
							if !r.In(page) || r.Dx() < 20 || r.Dy() < 20 {
								t.Errorf("%v %+v n=%d: tile %d at %v", size, look, n, i, r)
							}
							for _, o := range rs[i+1:] {
								if r.Overlaps(o) {
									t.Errorf("%v %+v n=%d: %v overlaps %v", size, look, n, r, o)
								}
							}
						}
						if a != "hero" || n == 1 {
							for i := 1; i < n; i++ {
								if area(rs[i]) > area(rs[0]) {
									t.Errorf("%v %+v n=%d: tile %d bigger than the first", size, look, n, i)
								}
							}
						}
					}
				}
			}
		}
	}
}

func area(r image.Rectangle) int { return r.Dx() * r.Dy() }

// The same seed gives the same mosaic; other seeds give others.
func TestMosaicFollowsTheSeed(t *testing.T) {
	a := Rects(7, DefaultSize, Look{Arrange: "mosaic", Seed: 20261114})
	b := Rects(7, DefaultSize, Look{Arrange: "mosaic", Seed: 20261114})
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("same seed, different mosaic")
		}
	}
	differ := false
	for s := range uint64(10) {
		c := Rects(7, DefaultSize, Look{Arrange: "mosaic", Seed: s})
		for i := range a {
			differ = differ || a[i] != c[i]
		}
	}
	if !differ {
		t.Error("the mosaic never changes with the seed")
	}
}

// A sample tile's disc sits whole in the middle of its picture, in every
// tile of every layout: never cut off by the tile's edge.
func TestSampleDiscIsWhole(t *testing.T) {
	disc := color.RGBA{196, 62, 50, 255}
	for _, look := range []Look{{}, {Arrange: "even"}, {Arrange: "mosaic", Seed: 3}, {Shape: "circle"}, {Shape: "arch"}} {
		for n := 1; n <= MaxTiles; n++ {
			tiles := make([]Tile, n)
			for i := range tiles {
				tiles[i] = Tile{Title: "Sample", Meta: "PREVIEW", Ink: color.RGBA{36, 66, 140, 255}, Disc: disc}
			}
			page := Render(tiles, DefaultSize, look)
			for i, r := range Rects(n, DefaultSize, look) {
				// the disc's bounds within the tile
				box := image.Rectangle{}
				for y := r.Min.Y; y < r.Max.Y; y++ {
					for x := r.Min.X; x < r.Max.X; x++ {
						if page.RGBAAt(x, y) == disc {
							box = box.Union(image.Rect(x, y, x+1, y+1))
						}
					}
				}
				if box.Empty() || box.Min.X <= r.Min.X || box.Max.X >= r.Max.X || box.Min.Y <= r.Min.Y || box.Dx() < box.Dy()-2 || box.Dy() < box.Dx()-2 {
					t.Errorf("%+v n=%d: tile %d at %v has its disc at %v", look, n, i, r, box)
				}
			}
		}
	}
}

// Shapes leave the background in the corners and the picture in the middle.
func TestShapesMaskTheCorners(t *testing.T) {
	tile := []Tile{{Image: image.NewRGBA(image.Rect(0, 0, 400, 300))}}
	for _, look := range []Look{{Shape: "rounded"}, {Shape: "circle"}, {Shape: "arch"}, {Shape: "circle", Background: "black"}} {
		page := Render(tile, DefaultSize, look)
		bg, _ := look.colours()
		r := Rects(1, DefaultSize, look)[0]
		if page.RGBAAt(r.Min.X+1, r.Min.Y+1) != bg {
			t.Errorf("%+v: top left corner is %v, want the background", look, page.RGBAAt(r.Min.X+1, r.Min.Y+1))
		}
		c := image.Pt((r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2)
		if page.RGBAAt(c.X, c.Y) == bg {
			t.Errorf("%+v: the middle is background", look)
		}
	}
	page := Render(tile, DefaultSize, Look{})
	r := Rects(1, DefaultSize, Look{})[0]
	if page.RGBAAt(r.Min.X+1, r.Min.Y+1) == paper {
		t.Error("a rectangle lost its corner")
	}
}

// In a cascade each tile is smaller than the one before.
func TestCascadeGetsSmaller(t *testing.T) {
	for _, size := range []image.Point{DefaultSize, {1200, 1600}} {
		for n := 2; n <= MaxTiles; n++ {
			rs := Rects(n, size, Look{Arrange: "cascade"})
			for i := 1; i < n; i++ {
				if area(rs[i]) >= area(rs[i-1]) {
					t.Errorf("%v n=%d: tile %d (%v) not smaller than tile %d (%v)", size, n, i, rs[i], i-1, rs[i-1])
				}
			}
		}
	}
}
