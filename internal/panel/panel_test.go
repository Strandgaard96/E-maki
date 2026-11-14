package panel

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"os"
	"testing"
)

func load(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestPaletteMatchesFugleramme(t *testing.T) {
	// PALETTE_6 as printed by Fugleramme's dither.py.
	want := [6][3]uint8{{0, 0, 0}, {208, 209, 210}, {231, 222, 35}, {205, 36, 37}, {30, 29, 174}, {29, 173, 35}}
	for i, c := range Palette {
		r, g, b, _ := c.RGBA()
		if got := [3]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}; got != want[i] {
			t.Errorf("palette[%d] = %v, want %v", i, got, want[i])
		}
	}
}

// The port reproduces Pillow's integer Floyd-Steinberg and palette cache, so
// it must match Fugleramme's output pixel for pixel.
func TestDitherMatchesFugleramme(t *testing.T) {
	got := Dither(load(t, "testdata/dither_input.png"))
	want := load(t, "testdata/dither_expected.png").(*image.Paletted)
	if got.Bounds() != want.Bounds() {
		t.Fatalf("bounds %v, want %v", got.Bounds(), want.Bounds())
	}
	if !bytes.Equal(got.Pix, want.Pix) {
		var off int
		for i := range got.Pix {
			if got.Pix[i] != want.Pix[i] {
				off++
			}
		}
		t.Errorf("%d of %d pixels differ from Fugleramme", off, len(got.Pix))
	}
}

func testPage() *image.Paletted {
	p := image.NewPaletted(image.Rect(0, 0, 8, 4), Palette)
	for y := 0; y < 4; y++ {
		for x := 0; x < 8; x++ {
			p.Pix[y*p.Stride+x] = uint8((x + 3*y) % 6)
		}
	}
	return p
}

func TestEncodeE6MatchesFugleramme(t *testing.T) {
	for _, tc := range []struct {
		file           string
		rotation, poll int
	}{{"testdata/e6_expected.bin", 0, 15}, {"testdata/e6_expected_rot90.bin", 90, 0}} {
		want, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatal(err)
		}
		// The fixtures are a small page with the 13.3" two-controller layout.
		defer func(g image.Point) { twoControllers = g }(twoControllers)
		twoControllers = rotateCCW(testPage(), tc.rotation).Bounds().Size()
		if got := EncodeE6(testPage(), tc.rotation, tc.poll); !bytes.Equal(got, want) {
			t.Errorf("%s: got %x, want %x", tc.file, got, want)
		}
	}
}

// The 7.3" glass has one controller: rows top to bottom, two pixels a byte.
// Any other glass has no .e6.
func TestEncodeE6OneController(t *testing.T) {
	defer func(g image.Point) { oneController = g }(oneController)
	oneController = image.Pt(8, 4)
	got := EncodeE6(testPage(), 0, 5)
	if w, h := binary.LittleEndian.Uint16(got[8:]), binary.LittleEndian.Uint16(got[10:]); w != 8 || h != 4 || len(got) != 20+16 {
		t.Fatalf("header %dx%d, %d bytes", w, h, len(got))
	}
	// Row 0 is palette 0 1 2 3 4 5 0 1: codes 0 1 2 3 5 6 0 1.
	if want := []byte{0x01, 0x23, 0x56, 0x01}; !bytes.Equal(got[20:24], want) {
		t.Errorf("row 0: %x, want %x", got[20:24], want)
	}
	if EncodeE6(testPage(), 90, 5) != nil {
		t.Error("a 4x8 glass got an .e6")
	}
}
