package collage

import (
	"image/color"
	"testing"
)

// An empty page is not blank paper: it carries the mark and a line of text.
func TestEmptyPageHasTheMark(t *testing.T) {
	page := Render(nil, DefaultSize)
	inks := map[color.RGBA]bool{}
	for y := 0; y < DefaultSize.Y; y += 4 {
		for x := 0; x < DefaultSize.X; x += 4 {
			inks[page.RGBAAt(x, y)] = true
		}
	}
	for _, c := range marks {
		if !inks[c] {
			t.Errorf("ink %v missing from the empty page", c)
		}
	}
}
