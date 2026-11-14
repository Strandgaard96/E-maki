// Package imagegen turns a topic's visual prompt into a picture. This is the
// only place data leaves the house, and only the checked English prompt does.
package imagegen

import (
	"context"
	"hash/fnv"
	"image"
	"image/color"
	"image/draw"
)

// Provider generates an image for a prompt.
type Provider interface {
	Generate(ctx context.Context, prompt string) (image.Image, error)
}

// Placeholder draws a flat block in one of the panel's inks, picked from the
// prompt, so the pipeline runs without an image API.
type Placeholder struct{}

var inks = []color.RGBA{{36, 66, 140, 255}, {196, 62, 50, 255}, {232, 186, 52, 255}, {52, 112, 72, 255}, {24, 22, 22, 255}}

func (Placeholder) Generate(_ context.Context, prompt string) (image.Image, error) {
	h := fnv.New32a()
	h.Write([]byte(prompt))
	n := h.Sum32()
	bg, fg := inks[n%uint32(len(inks))], inks[(n/7+1)%uint32(len(inks))]
	if fg == bg {
		fg = color.RGBA{252, 250, 244, 255}
	}
	img := image.NewRGBA(image.Rect(0, 0, 600, 600))
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	// a centred disc, so tiles read as pictures rather than flat colour
	for y := 0; y < 600; y++ {
		for x := 0; x < 600; x++ {
			if dx, dy := x-300, y-300; dx*dx+dy*dy < 170*170 {
				img.Set(x, y, fg)
			}
		}
	}
	return img, nil
}
