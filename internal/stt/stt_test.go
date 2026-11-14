package stt

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func TestWhisperAndRouter(t *testing.T) {
	var calls []string
	srv := func(text, lang string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/inference" {
				t.Errorf("path %s", r.URL.Path)
			}
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatal(err)
			}
			calls = append(calls, r.FormValue("language"))
			if r.FormValue("no_language_probabilities") != "true" {
				t.Error("asked for language probabilities")
			}
			json.NewEncoder(w).Encode(map[string]string{"text": text, "language": lang})
		}))
	}
	detect := srv(" rough text ", "danish")
	defer detect.Close()
	edda := srv("præcis dansk tekst", "danish")
	defer edda.Close()

	r := Router{Detect: Whisper{URL: detect.URL}, ByLang: map[string]Transcriber{"da": Whisper{URL: edda.URL}}}
	seg, err := r.Transcribe(context.Background(), make([]int16, 1600), "")
	if err != nil {
		t.Fatal(err)
	}
	if seg.Text != "præcis dansk tekst" || seg.Lang != "da" {
		t.Fatalf("got %+v", seg)
	}
	if len(calls) != 2 || calls[0] != "auto" || calls[1] != "da" {
		t.Fatalf("calls %v, want [auto da]", calls)
	}
}

// fake answers with lang when asked for "auto", else with what it was asked.
type fake struct {
	name  string
	guess string
	calls *[]string
}

func (f fake) Transcribe(_ context.Context, _ []int16, lang string) (Segment, error) {
	*f.calls = append(*f.calls, f.name+":"+lang)
	if lang == "" {
		lang = f.guess
	}
	return Segment{Text: f.name, Lang: lang}, nil
}

func TestNoSpeech(t *testing.T) {
	var calls []string
	r := Router{Detect: fake{"", "en", &calls}, ByLang: map[string]Transcriber{"da": fake{"edda", "da", &calls}}}
	seg, err := r.In(context.Background(), nil, []string{"da", "en"})
	if err != nil || seg.Text != "" || len(calls) != 1 {
		t.Fatalf("got %+v, %v from calls %v; want one call and no text", seg, err, calls)
	}
}

func TestLanguagesSpoken(t *testing.T) {
	for _, c := range []struct {
		name  string
		guess string
		langs []string
		want  string // the server that wrote the text
		calls string
	}{
		{"any language", "sv", nil, "stock", "stock:"},
		{"one language skips detection", "sv", []string{"da"}, "edda", "edda:da"},
		{"one language without its own server", "sv", []string{"en"}, "stock", "stock:en"},
		{"guess in the list", "en", []string{"da", "en"}, "stock", "stock:"},
		{"guess in the list with a server", "da", []string{"en", "da"}, "edda", "stock: edda:da"},
		{"guess outside the list", "sv", []string{"da", "en"}, "edda", "stock: edda:da"},
		{"outside the list, first has no server", "sv", []string{"en", "da"}, "stock", "stock: stock:en"},
	} {
		var calls []string
		r := Router{Detect: fake{"stock", c.guess, &calls}, ByLang: map[string]Transcriber{"da": fake{"edda", "da", &calls}}}
		seg, err := r.In(context.Background(), nil, c.langs)
		if err != nil {
			t.Fatal(err)
		}
		if seg.Text != c.want || strings.Join(calls, " ") != c.calls {
			t.Errorf("%s: got %+v from calls %v, want %s from %s", c.name, seg, calls, c.want, c.calls)
		}
		if len(c.langs) > 0 && !slices.Contains(c.langs, seg.Lang) {
			t.Errorf("%s: language %q is not one of %v", c.name, seg.Lang, c.langs)
		}
	}
}

func TestLangCode(t *testing.T) {
	for in, want := range map[string]string{"German": "de", "danish": "da", "nynorsk": "no", "haw": "haw", "cantonese": "yue"} {
		if got := langCode(in); got != want {
			t.Errorf("langCode(%q) = %q, want %q", in, got, want)
		}
	}
}
