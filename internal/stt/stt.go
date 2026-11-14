// Package stt turns speech segments into text through a speech-to-text
// sidecar (whisper.cpp's whisper-server). Audio is sent in memory over the
// local network and never stored.
package stt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/Strandgaard96/e-maki/internal/audio"
)

// Segment is one transcribed stretch of speech.
type Segment struct {
	Text string
	Lang string // ISO 639-1, e.g. "da", "no", "en"
}

// Transcriber turns 16 kHz mono PCM into text. lang "" means auto-detect.
type Transcriber interface {
	Transcribe(ctx context.Context, pcm []int16, lang string) (Segment, error)
}

// defaultClient gives up on a hung sidecar instead of stopping the pipeline.
var defaultClient = &http.Client{Timeout: 5 * time.Minute}

// Whisper talks to a whisper.cpp server (POST /inference).
type Whisper struct {
	URL    string // e.g. http://localhost:8081
	Client *http.Client
}

func (w Whisper) Transcribe(ctx context.Context, pcm []int16, lang string) (Segment, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "segment.wav")
	fw.Write(audio.EncodeWAV(pcm))
	if lang == "" {
		lang = "auto"
	}
	mw.WriteField("language", lang)
	mw.WriteField("response_format", "verbose_json")
	mw.WriteField("temperature", "0")
	// E-maki does not read the language probabilities. Asking for them costs
	// a second language detection, and whisper-server v1.9.5 answers 500
	// when voice detection found no speech (fixed upstream after 1.9.5,
	// ggml-org/whisper.cpp#4158).
	mw.WriteField("no_language_probabilities", "true")
	mw.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(w.URL, "/")+"/inference", &body)
	if err != nil {
		return Segment{}, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c := w.Client
	if c == nil {
		c = defaultClient
	}
	resp, err := c.Do(req)
	if err != nil {
		return Segment{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Segment{}, fmt.Errorf("whisper-server: %s", resp.Status)
	}
	var out struct {
		Text     string `json:"text"`
		Language string `json:"language"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return Segment{}, err
	}
	return Segment{Text: strings.TrimSpace(out.Text), Lang: langCode(out.Language)}, nil
}

// Languages are the languages Whisper knows, by code and by the name
// whisper-server answers with (whisper.cpp's g_lang).
var Languages = [][2]string{
	{"en", "english"}, {"zh", "chinese"}, {"de", "german"}, {"es", "spanish"}, {"ru", "russian"},
	{"ko", "korean"}, {"fr", "french"}, {"ja", "japanese"}, {"pt", "portuguese"}, {"tr", "turkish"},
	{"pl", "polish"}, {"ca", "catalan"}, {"nl", "dutch"}, {"ar", "arabic"}, {"sv", "swedish"},
	{"it", "italian"}, {"id", "indonesian"}, {"hi", "hindi"}, {"fi", "finnish"}, {"vi", "vietnamese"},
	{"he", "hebrew"}, {"uk", "ukrainian"}, {"el", "greek"}, {"ms", "malay"}, {"cs", "czech"},
	{"ro", "romanian"}, {"da", "danish"}, {"hu", "hungarian"}, {"ta", "tamil"}, {"no", "norwegian"},
	{"th", "thai"}, {"ur", "urdu"}, {"hr", "croatian"}, {"bg", "bulgarian"}, {"lt", "lithuanian"},
	{"la", "latin"}, {"mi", "maori"}, {"ml", "malayalam"}, {"cy", "welsh"}, {"sk", "slovak"},
	{"te", "telugu"}, {"fa", "persian"}, {"lv", "latvian"}, {"bn", "bengali"}, {"sr", "serbian"},
	{"az", "azerbaijani"}, {"sl", "slovenian"}, {"kn", "kannada"}, {"et", "estonian"}, {"mk", "macedonian"},
	{"br", "breton"}, {"eu", "basque"}, {"is", "icelandic"}, {"hy", "armenian"}, {"ne", "nepali"},
	{"mn", "mongolian"}, {"bs", "bosnian"}, {"kk", "kazakh"}, {"sq", "albanian"}, {"sw", "swahili"},
	{"gl", "galician"}, {"mr", "marathi"}, {"pa", "punjabi"}, {"si", "sinhala"}, {"km", "khmer"},
	{"sn", "shona"}, {"yo", "yoruba"}, {"so", "somali"}, {"af", "afrikaans"}, {"oc", "occitan"},
	{"ka", "georgian"}, {"be", "belarusian"}, {"tg", "tajik"}, {"sd", "sindhi"}, {"gu", "gujarati"},
	{"am", "amharic"}, {"yi", "yiddish"}, {"lo", "lao"}, {"uz", "uzbek"}, {"fo", "faroese"},
	{"ht", "haitian creole"}, {"ps", "pashto"}, {"tk", "turkmen"}, {"nn", "nynorsk"}, {"mt", "maltese"},
	{"sa", "sanskrit"}, {"lb", "luxembourgish"}, {"my", "myanmar"}, {"bo", "tibetan"}, {"tl", "tagalog"},
	{"mg", "malagasy"}, {"as", "assamese"}, {"tt", "tatar"}, {"haw", "hawaiian"}, {"ln", "lingala"},
	{"ha", "hausa"}, {"ba", "bashkir"}, {"jw", "javanese"}, {"su", "sundanese"}, {"yue", "cantonese"},
}

// Known reports whether code is one of Whisper's language codes.
func Known(code string) bool {
	return slices.ContainsFunc(Languages, func(l [2]string) bool { return l[0] == code })
}

// langCode accepts either a code or whisper's full language name.
// Nynorsk counts as Norwegian, so one Norwegian model gets both.
func langCode(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "nynorsk" || s == "nn" {
		return "no"
	}
	for _, l := range Languages {
		if s == l[1] {
			return l[0]
		}
	}
	return s
}

// Router detects the language with Detect, then re-transcribes with the
// language's fine-tuned model when one is configured (Edda for Danish,
// NB-Whisper for Norwegian).
type Router struct {
	Detect Transcriber
	ByLang map[string]Transcriber
}

func (r Router) Transcribe(ctx context.Context, pcm []int16, lang string) (Segment, error) {
	if lang == "" {
		return r.In(ctx, pcm, nil)
	}
	return r.In(ctx, pcm, []string{lang})
}

// In transcribes speech in one of langs, the languages spoken in the house.
// One language skips detection. With several, a guess outside the list
// (small models hear Danish as Swedish) is redone in the first one. No
// languages accepts any guess.
func (r Router) In(ctx context.Context, pcm []int16, langs []string) (Segment, error) {
	if len(langs) == 1 {
		return r.forced(ctx, pcm, langs[0])
	}
	seg, err := r.Detect.Transcribe(ctx, pcm, "")
	if err != nil || seg.Text == "" {
		// No words (whisper's voice detection found no speech): nothing to
		// redo in another language or on another server.
		return seg, err
	}
	if len(langs) > 1 && !slices.Contains(langs, seg.Lang) {
		return r.forced(ctx, pcm, langs[0])
	}
	if t, ok := r.ByLang[seg.Lang]; ok {
		return t.Transcribe(ctx, pcm, seg.Lang)
	}
	return seg, nil
}

// forced transcribes in lang, on its own server when it has one.
func (r Router) forced(ctx context.Context, pcm []int16, lang string) (Segment, error) {
	t, ok := r.ByLang[lang]
	if !ok {
		t = r.Detect
	}
	seg, err := t.Transcribe(ctx, pcm, lang)
	seg.Lang = lang
	return seg, err
}
