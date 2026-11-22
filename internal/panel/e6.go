package panel

import (
	"encoding/binary"
	"hash/crc32"
	"image"
)

// The .e6 format (ported from Fugleramme's e6.py, MIT): a 20-byte
// little-endian header, then the controllers' colour codes two pixels a byte
// (left pixel in the high nibble). The 7.3" glass has one controller, which
// reads its landscape rows top to bottom. The 13.3" glass has two: it is
// scanned a quarter turn clockwise, and the payload is every row's left half
// followed by every row's right half. Other glasses have no .e6.
//
//	0 magic "E6F1"  4 version u16  6 encoding u8  7 poll minutes u8
//	8 width u16     10 height u16  12 payload length u32  16 CRC-32 u32
const (
	e6Version  = 1
	e6Encoding = 1
)

// native maps palette indices to the controllers' codes, which skip 4.
var native = [6]byte{0, 1, 2, 3, 5, 6}

// The glasses with an .e6 layout, by their landscape size (Fugleramme's
// GLASSES).
var (
	oneController  = image.Pt(800, 480)   // 7.3"
	twoControllers = image.Pt(1600, 1200) // 13.3"
)

// EncodeE6 packs a dithered page (as the viewer sees it) for the glass, or
// returns nil for a glass without an .e6 layout or a page not in 6 inks.
// rotation is the frame's rotation in degrees counter-clockwise (0, 90, 180, 270).
func EncodeE6(page *image.Paletted, rotation, poll int) []byte {
	if len(page.Palette) != len(native) {
		return nil
	}
	glass := rotateCCW(page, rotation)
	halves := []int{0}
	switch glass.Bounds().Size() {
	case oneController:
	case twoControllers:
		glass = rotateCCW(glass, 270) // a quarter turn clockwise
		halves = []int{0, glass.Bounds().Dx() / 2}
	default:
		return nil
	}
	w, h := glass.Bounds().Dx(), glass.Bounds().Dy()
	half := w / len(halves)
	payload := make([]byte, 0, w*h/2)
	for _, x0 := range halves {
		for y := 0; y < h; y++ {
			for x := x0; x < x0+half; x += 2 {
				hi := native[glass.Pix[y*glass.Stride+x]]
				lo := native[glass.Pix[y*glass.Stride+x+1]]
				payload = append(payload, hi<<4|lo)
			}
		}
	}
	if poll > 255 {
		poll = 255
	}
	hdr := make([]byte, 20)
	copy(hdr, "E6F1")
	binary.LittleEndian.PutUint16(hdr[4:], e6Version)
	hdr[6] = e6Encoding
	hdr[7] = byte(poll)
	binary.LittleEndian.PutUint16(hdr[8:], uint16(w))
	binary.LittleEndian.PutUint16(hdr[10:], uint16(h))
	binary.LittleEndian.PutUint32(hdr[12:], uint32(len(payload)))
	binary.LittleEndian.PutUint32(hdr[16:], crc32.ChecksumIEEE(payload))
	return append(hdr, payload...)
}

// rotateCCW turns a paletted image counter-clockwise by deg (a multiple of 90),
// matching PIL's Image.Transpose.ROTATE_<deg>.
func rotateCCW(src *image.Paletted, deg int) *image.Paletted {
	deg = ((deg % 360) + 360) % 360
	if deg == 0 {
		return src
	}
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	ow, oh := h, w
	if deg == 180 {
		ow, oh = w, h
	}
	dst := image.NewPaletted(image.Rect(0, 0, ow, oh), src.Palette)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var nx, ny int
			switch deg {
			case 90:
				nx, ny = y, w-1-x
			case 180:
				nx, ny = w-1-x, h-1-y
			case 270:
				nx, ny = h-1-y, x
			}
			dst.Pix[ny*dst.Stride+nx] = src.Pix[y*src.Stride+x]
		}
	}
	return dst
}
