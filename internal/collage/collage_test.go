package collage

import (
	"bytes"
	"context"
	"image"
	"testing"

	"github.com/Strandgaard96/e-maki/internal/imagegen"
)

func TestRenderIsDeterministic(t *testing.T) {
	var tiles []Tile
	for _, p := range []string{"hot dog", "ski trip", "rødgrød med fløde"} {
		img, _ := imagegen.Placeholder{}.Generate(context.Background(), p)
		tiles = append(tiles, Tile{Title: p, Meta: "KITCHEN · SUN", Image: img})
	}
	a, b := Render(tiles, DefaultSize), Render(tiles, DefaultSize)
	if a.Bounds().Size() != DefaultSize {
		t.Fatalf("size %v", a.Bounds())
	}
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Fatal("same input rendered differently")
	}
}

// Every tile count has its own layout, with each cell of the grid used
// exactly once: no holes and no overlaps.
func TestLayoutFits(t *testing.T) {
	for n := 1; n <= MaxTiles; n++ {
		l := layouts[n-1]
		if len(l.cells) != n {
			t.Errorf("%d tiles: layout of %d cells", n, len(l.cells))
			continue
		}
		used := map[[2]int]int{}
		for _, c := range l.cells {
			for x := c.c; x < c.c+c.cs; x++ {
				for y := c.r; y < c.r+c.rs; y++ {
					used[[2]int{x, y}]++
				}
			}
		}
		for x := range l.cols {
			for y := range l.rows {
				if used[[2]int{x, y}] != 1 {
					t.Errorf("%d tiles: grid place %d,%d used %d times", n, x, y, used[[2]int{x, y}])
				}
			}
		}
		if len(used) != l.cols*l.rows {
			t.Errorf("%d tiles: cells outside the grid", n)
		}
	}
}

func TestFitScalesDownOnly(t *testing.T) {
	big := image.NewRGBA(image.Rect(0, 0, 4000, 3000))
	if b := Fit(big, DefaultSize).Bounds(); b.Dx() >= 4000 || b.Dy() < DefaultSize.X {
		t.Errorf("big picture fit to %v", b)
	}
	small := image.NewRGBA(image.Rect(0, 0, 1024, 1024))
	if Fit(small, DefaultSize) != image.Image(small) {
		t.Error("small picture was scaled")
	}
}

func TestCaptionsStayInsideTheirTile(t *testing.T) {
	// The longest label the topic check lets through, on the narrowest tile.
	long := "midnight hardware store run for one missing shelf bracket screw"
	for _, size := range []float64{26, 40, 52} {
		for _, width := range []int{358, 742, 1512} {
			s, fitted := fit(long, size, width)
			if w := textWidth(s, fitted); w > width {
				t.Errorf("size %v in %d px: %q is %d px wide", size, width, s, w)
			}
			if fitted < size*0.75 {
				t.Errorf("size %v in %d px: shrunk to %v", size, width, fitted)
			}
		}
	}
	if s, size := fit("ski trip", 26, 358); s != "ski trip" || size != 26 {
		t.Errorf("a short caption changed: %q at %v", s, size)
	}
}
