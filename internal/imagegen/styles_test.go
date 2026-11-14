package imagegen

import (
	"strings"
	"testing"
)

func TestStylePrompt(t *testing.T) {
	linocut := Styles[3]
	for in, want := range map[string]string{
		"":               DefaultStyle,
		linocut.Name:     linocut.Prompt,
		"linocut":        linocut.Prompt,
		linocut.Prompt:   linocut.Prompt,
		"woodcut, sepia": "woodcut, sepia",
	} {
		if got := StylePrompt(in); got != want {
			t.Errorf("StylePrompt(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"": "Comic", "LINOCUT": "Linocut", linocut.Prompt: "Linocut", "woodcut": "Your own"} {
		if got := StyleName(in); got != want {
			t.Errorf("StyleName(%q) = %q, want %q", in, got, want)
		}
	}
}

// Every preset keeps to the panel's inks and asks for no text.
func TestPresetsKeepToTheInks(t *testing.T) {
	names := map[string]bool{}
	for _, s := range Styles {
		if names[s.Name] || !strings.HasSuffix(s.Prompt, panelInks) {
			t.Errorf("%s: duplicate name or missing the ink rules", s.Name)
		}
		names[s.Name] = true
	}
}

func TestAddStyles(t *testing.T) {
	saved := Styles
	t.Cleanup(func() { Styles, DefaultStyle = saved, saved[0].Prompt })
	Styles = append([]Style(nil), saved...)
	AddStyles([]Style{{"comic", "my comic"}, {"Woodcut", "woodcut print"}})
	if DefaultStyle != "my comic" || StylePrompt("woodcut") != "woodcut print" || len(Styles) != len(saved)+1 {
		t.Errorf("got %d styles, default %q", len(Styles), DefaultStyle)
	}
}
