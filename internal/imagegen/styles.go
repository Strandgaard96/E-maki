package imagegen

import (
	"slices"
	"strings"
)

// Style is a named art style the settings page offers.
type Style struct {
	Name   string `json:"name"`
	Prompt string `json:"prompt"`
}

// panelInks is added to every preset. A Spectra 6 panel has six inks and no
// greys, so anything else (gradients, pastels, skin tones, fine texture) is
// dithered into speckle, and a picture is often only a small collage tile.
const panelInks = "Use only flat areas of pure black, white, red, yellow, blue and green, " +
	"and draw everything in those six colours, even skin, wood and sky: " +
	"no gradients, shading, soft shadows, glow, texture, pastel or muted tones. " +
	"Show one large, instantly recognisable subject that fills most of the picture against a plain background, " +
	"with strong contrast between neighbouring shapes, so it still reads as a small picture across the room. " +
	"No text, letters, numbers, logos, signatures, borders or frames."

// Styles are the presets the settings page offers; a profile can add its own
// (image_styles) and anything else can be typed in. Each one describes the
// look in a sentence, which the Gemini image models follow better than a
// list of keywords.
var Styles = []Style{
	{"Comic", "Draw it as a single comic-book panel in a bold cartoon style: thick black ink outlines, bright flat fills and funny, exaggerated expressions. " + panelInks},
	{"Clear line", "Draw it in the European clear-line comic style: even, unbroken black outlines of one weight, flat colour fills with no hatching, and a calm, tidy composition. " + panelInks},
	{"Picture book", "Draw it as a friendly children's picture-book illustration: rounded shapes, clean dark outlines and simple, cheerful characters. " + panelInks},
	{"Linocut", "Make it a hand-carved linocut print: chunky black shapes with rough, confident carved edges, white paper showing through, and two or three flat blocks of colour. " + panelInks},
	{"Paper cut-out", "Make it a cut-paper collage: big, simple shapes cut from flat sheets of coloured paper and laid on white paper, slightly overlapping, with crisp edges. " + panelInks},
	{"Stained glass", "Make it a stained-glass window: the scene split into large panes of flat, saturated colour held together by thick black lead lines. " + panelInks},
	{"Screen print", "Make it a mid-century screen-print illustration: strong simplified silhouettes, a few flat spot colours with crisp edges, and wide calm areas of sky or ground. " + panelInks},
	{"Pop art", "Make it a pop-art painting: heavy black outlines, big flat colour fields, playful and loud. " + panelInks},
	{"Ink and red", "Make it a black ink drawing on white paper with bold brush lines and solid black shadows, and red as the only colour, for the most important thing. " + panelInks},
	{"Pixel art", "Make it chunky retro pixel art: a low-resolution scene with big square pixels, a small palette and no anti-aliasing. " + panelInks},
}

// DefaultStyle is used when image_style is empty: the first preset.
var DefaultStyle = Styles[0].Prompt

// StylePrompt is the text added to every image prompt for image_style s:
// a preset's prompt when s names one, DefaultStyle when s is empty, and s
// itself otherwise.
func StylePrompt(s string) string {
	if s == "" {
		return DefaultStyle
	}
	for _, st := range Styles {
		if strings.EqualFold(st.Name, s) {
			return st.Prompt
		}
	}
	return s
}

// StyleName names image_style s for the settings page: a preset's name, or
// "Your own" for a style typed in.
func StyleName(s string) string {
	p := StylePrompt(s)
	for _, st := range Styles {
		if st.Prompt == p {
			return st.Name
		}
	}
	return "Your own"
}

// AddStyles adds a profile's own presets; one with a preset's name replaces
// it.
func AddStyles(own []Style) {
	for _, o := range own {
		i := slices.IndexFunc(Styles, func(st Style) bool { return strings.EqualFold(st.Name, o.Name) })
		if i < 0 {
			Styles = append(Styles, o)
		} else {
			Styles[i] = o
		}
	}
	DefaultStyle = Styles[0].Prompt
}
