package panel

import "testing"

func TestPopMatchesFugleramme(t *testing.T) {
	in := load(t, "testdata/dither_input.png")
	want := load(t, "testdata/pop_expected.png")
	w, h, px := pop(in)
	var off int
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := want.At(x, y).RGBA()
			c := px[y*w+x]
			if uint8(c[0]) != uint8(r>>8) || uint8(c[1]) != uint8(g>>8) || uint8(c[2]) != uint8(b>>8) {
				off++
			}
		}
	}
	if off != 0 {
		t.Errorf("%d of %d pixels differ from Fugleramme's _pop", off, w*h)
	}
}
