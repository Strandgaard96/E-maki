package collage

import (
	"bytes"
	"context"
	"testing"

	"github.com/Strandgaard96/e-kiru/internal/imagegen"
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

// Each tile count gets a layout with no empty cells where one exists.
func TestLayoutFits(t *testing.T) {
	for n, want := range map[int]int{1: 1, 2: 2, 3: 3, 4: 4, 5: 6, 6: 6, 7: 9, 9: 9} {
		for _, l := range layouts {
			if len(l.cells) >= n {
				if len(l.cells) != want {
					t.Errorf("%d tiles: layout of %d cells, want %d", n, len(l.cells), want)
				}
				break
			}
		}
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
