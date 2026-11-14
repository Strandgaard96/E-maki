package pipeline

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Strandgaard96/e-maki/internal/collage"
	"github.com/Strandgaard96/e-maki/internal/imagegen"
	"github.com/Strandgaard96/e-maki/internal/memory"
	"github.com/Strandgaard96/e-maki/internal/panel"
	"github.com/Strandgaard96/e-maki/internal/topic"
)

// feed is Feed and waiting for the pictures it asked for.
func feed(p *Pipeline, text string) error {
	defer p.Wait()
	return p.Feed(context.Background(), text)
}

// Synthetic conversation, not a real recording.
var lines = []string{
	"skal vi ikke tage til Lofoten og stå på ski i februar",
	"ja ski i Lofoten er fantastisk, the skiing there is great",
	"jeg har aldrig prøvet ski i Norge",
	"vi kunne leje en hytte, ski hele dagen",
	"har I set at katten er stukket af igen",
	"katten sad på hegnet i morges",
	"the neighbour's katten is always escaping",
	"katten kommer nok hjem til aftensmad",
}

func TestEndToEndWithFakes(t *testing.T) {
	now := time.Date(2026, 11, 8, 20, 0, 0, 0, time.UTC)
	srv := &panel.Server{}
	p := &Pipeline{
		Mem: memory.New(memory.Settings{HalfLife: 48 * time.Hour, MinConfidence: 0.5, MinMentions: 1,
			DailyImageCap: 10, MaxTiles: 9, MinWeight: 0.1, BlockFor: 24 * time.Hour}),
		Extractor: topic.Fake{}, Images: imagegen.Placeholder{}, Panel: srv,
		WindowSegments: 4, ExtractEvery: 4, Now: func() time.Time { return now },
	}
	for _, l := range lines {
		if err := feed(p, l); err != nil {
			t.Fatal(err)
		}
	}
	wall := p.Mem.Wall(now)
	if len(wall) != 2 || wall[0].Label != "katten" && wall[1].Label != "katten" {
		t.Fatalf("wall %v, want ski and katten", wall)
	}
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/frame.png", nil))
	if rec.Code != http.StatusOK || rec.Body.Len() == 0 {
		t.Fatalf("frame.png: %d", rec.Code)
	}

	p.Press("remove")
	if wall := p.Mem.Wall(now); len(wall) != 1 {
		t.Fatalf("after remove wall %v, want 1 tile", wall)
	}
}

type fixed topic.Result

// "pin" pins the newest tile and unpins it on a second press; "next" puts
// the next waiting picture up.
func TestPinAndNext(t *testing.T) {
	now := time.Date(2026, 11, 8, 20, 0, 0, 0, time.UTC)
	p := &Pipeline{
		Mem: memory.New(memory.Settings{HalfLife: time.Hour, MinConfidence: 0.5, MinMentions: 1,
			DailyImageCap: 10, MaxTiles: 9, MinWeight: 0.1, Every: time.Hour}),
		Images: imagegen.Placeholder{}, Panel: &panel.Server{},
		WindowSegments: 1, ExtractEvery: 1, Now: func() time.Time { return now },
	}
	for _, label := range []string{"garden gnome theft", "rollerblading date"} {
		p.Extractor = fixed{Topic: label, Confidence: 0.9, VisualPrompt: "a picture", Sensitive: "none"}
		if err := feed(p, "synthetic talk"); err != nil {
			t.Fatal(err)
		}
	}
	if w, q := p.Wall(), p.Waiting(); len(w) != 1 || len(q) != 1 {
		t.Fatalf("wall %v, queue %v: want one of each", w, q)
	}
	p.Press("pin")
	if !p.Wall()[0].Pinned {
		t.Fatal("pin did not pin the newest tile")
	}
	p.Press("pin")
	if p.Wall()[0].Pinned {
		t.Fatal("a second pin did not unpin it")
	}
	p.Press("next")
	if w, q := p.Wall(), p.Waiting(); len(w) != 2 || len(q) != 0 {
		t.Fatalf("after next: wall %v, queue %v", w, q)
	}
}

func (f fixed) Extract(context.Context, string, []string) (topic.Result, error) {
	return topic.Result(f), nil
}

// A rejected topic was too private to send, so its label is not logged
// either: only the reason is.
func TestRejectedLabelIsNotLogged(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	p := &Pipeline{
		Mem:       memory.New(memory.Settings{HalfLife: time.Hour, MinMentions: 1, DailyImageCap: 1, MaxTiles: 9}),
		Extractor: fixed{Topic: "Dentist visit", Confidence: 0.9, Sensitive: "health"},
		Images:    imagegen.Placeholder{}, Panel: &panel.Server{},
		WindowSegments: 1, ExtractEvery: 1, Now: time.Now, BlockSensitive: []string{"health"},
	}
	if err := feed(p, "synthetic line about the dentist appointment next week"); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); strings.Contains(got, "Dentist") || !strings.Contains(got, "sensitive topic (health)") {
		t.Errorf("log should give the reason without the label: %s", got)
	}
}

// The log names the topic model, so you can see which one answered.
func TestLogNamesTopicModel(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	p := &Pipeline{
		Mem:       memory.New(memory.Settings{HalfLife: time.Hour, MinMentions: 1, DailyImageCap: 1, MaxTiles: 9}),
		Extractor: named{fixed{Topic: "Ski trip", Confidence: 0.9, VisualPrompt: "skis", Sensitive: "none"}},
		Images:    imagegen.Placeholder{}, Panel: &panel.Server{},
		WindowSegments: 1, ExtractEvery: 1, Now: time.Now,
	}
	if err := feed(p, "synthetic line about skiing"); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); !strings.Contains(got, "llm (model-x at api.example.com): ") {
		t.Errorf("log should name the topic model: %s", got)
	}
}

type named struct{ fixed }

func (named) String() string { return "model-x at api.example.com" }

// A topic on the never-paint list gets no picture, and the word is not logged.
func TestNeverPaint(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	p := &Pipeline{
		Mem:       memory.New(memory.Settings{HalfLife: time.Hour, MinMentions: 1, DailyImageCap: 1, MaxTiles: 9}),
		Extractor: fixed{Topic: "Quarterly budget meeting", VisualPrompt: "people around a table", Confidence: 0.9, Sensitive: "none"},
		Images:    imagegen.Placeholder{}, Panel: &panel.Server{},
		WindowSegments: 1, ExtractEvery: 1, Now: time.Now, NeverPaint: []string{"meeting"},
	}
	if err := feed(p, "synthetic line about work"); err != nil {
		t.Fatal(err)
	}
	if n := p.ImagesToday(); n != 0 {
		t.Errorf("%d pictures made, want none", n)
	}
	if got := buf.String(); strings.Contains(got, "meeting") || !strings.Contains(got, "never-paint") {
		t.Errorf("log should give the reason without the word: %s", got)
	}
}

// Topics that fade off the wall are forgotten together with their image.
func TestFadedTopicsAreForgotten(t *testing.T) {
	now := time.Date(2026, 11, 8, 20, 0, 0, 0, time.UTC)
	next := fixed{Topic: "garden gnome theft", Confidence: 0.9, VisualPrompt: "a gnome", Sensitive: "none"}
	p := &Pipeline{
		Mem: memory.New(memory.Settings{HalfLife: time.Hour, MinConfidence: 0.5, MinMentions: 1,
			DailyImageCap: 10, MaxTiles: 9, MinWeight: 0.1}),
		Extractor: next, Images: imagegen.Placeholder{}, Panel: &panel.Server{},
		WindowSegments: 1, ExtractEvery: 1, Now: func() time.Time { return now },
	}
	if err := feed(p, "synthetic talk"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(24 * time.Hour) // 24 half-lives later
	p.Extractor = fixed{Topic: "burnt lasagne", Confidence: 0.9, VisualPrompt: "a lasagne", Sensitive: "none"}
	if err := feed(p, "synthetic talk"); err != nil {
		t.Fatal(err)
	}
	if len(p.images) != 1 || p.images["burnt lasagne"] == nil || len(p.Mem.Recent(9)) != 1 {
		t.Fatalf("images %d, topics %v: the faded topic should be gone", len(p.images), p.Mem.Recent(9))
	}
}

type seen struct{ window *string }

func (s seen) Extract(_ context.Context, window string, _ []string) (topic.Result, error) {
	*s.window = window
	return topic.Result{}, nil
}

// Quiet segments age the window, so an old topic does not linger after
// the room goes quiet.
func TestQuietSegmentsAgeTheWindow(t *testing.T) {
	var got string
	p := &Pipeline{
		Mem:       memory.New(memory.Settings{HalfLife: time.Hour, MinConfidence: 0.5, MinMentions: 1, MaxTiles: 9}),
		Extractor: seen{&got}, Images: imagegen.Placeholder{}, Panel: &panel.Server{},
		WindowSegments: 3, ExtractEvery: 1, Now: time.Now,
	}
	for _, text := range []string{"synthetic old talk about basketball", "", "", "synthetic new talk about cats"} {
		if err := feed(p, text); err != nil {
			t.Fatal(err)
		}
	}
	if got != "synthetic new talk about cats" {
		t.Fatalf("window %q, want only the new talk", got)
	}
}

// The web page pins and removes tiles, and its settings apply at once.
func TestPinRemoveConfigure(t *testing.T) {
	now := time.Date(2026, 11, 8, 20, 0, 0, 0, time.UTC)
	mem := memory.Settings{HalfLife: time.Hour, MinConfidence: 0.5, MinMentions: 1, DailyImageCap: 10, MaxTiles: 9, MinWeight: 0.1, BlockFor: time.Hour}
	srv := &panel.Server{}
	p := &Pipeline{
		Mem: memory.New(mem), Images: imagegen.Placeholder{}, Panel: srv,
		WindowSegments: 1, ExtractEvery: 1, Now: func() time.Time { return now },
	}
	for _, label := range []string{"garden gnome theft", "burnt lasagne"} {
		p.Extractor = fixed{Topic: label, Confidence: 0.9, VisualPrompt: "a " + label, Sensitive: "none"}
		if err := feed(p, "synthetic talk"); err != nil {
			t.Fatal(err)
		}
	}
	if !p.Pin("Garden gnome theft", true) || p.Pin("no such topic", true) {
		t.Fatal("pin should find the tile by its label, in any case, and only it")
	}
	now = now.Add(24 * time.Hour)
	if w := p.Wall(); len(w) != 1 || w[0].Label != "garden gnome theft" || !w[0].Pinned {
		t.Fatalf("a day later only the pinned tile stays: %v", w)
	}
	if !p.Remove("garden gnome theft") || len(p.Wall()) != 0 {
		t.Fatalf("remove: wall %v", p.Wall())
	}
	etag := srv.ETag()
	mem.MinMentions = 2
	if err := p.Configure(Options{Extractor: p.Extractor, WindowSegments: 1, ExtractEvery: 1, Images: imagegen.Placeholder{}, BlockSensitive: []string{"health"}, BlockNames: true}, mem); err != nil {
		t.Fatal(err)
	}
	if srv.ETag() == "" || !p.BlockNames {
		t.Fatalf("configure: etag %q (was %q), block names %v", srv.ETag(), etag, p.BlockNames)
	}
	p.Extractor = fixed{Topic: "squeaky door", Confidence: 0.9, VisualPrompt: "a door", Sensitive: "none"}
	if err := feed(p, "synthetic talk"); err != nil {
		t.Fatal(err)
	}
	if len(p.Wall()) != 0 {
		t.Fatal("with MinMentions 2 a topic seen once is not on the wall yet")
	}
}

// With fewer pictures than the layout has places, the preview fills the rest
// with samples, so a layout change always shows. The wall itself is untouched.
func TestDraftFillsEmptyPlaces(t *testing.T) {
	now := time.Date(2026, 11, 8, 20, 0, 0, 0, time.UTC)
	set := memory.Settings{HalfLife: 48 * time.Hour, MinConfidence: 0.5, MinMentions: 1,
		DailyImageCap: 10, MaxTiles: 4, MinWeight: 0.1, BlockFor: 24 * time.Hour}
	p := &Pipeline{Mem: memory.New(set), Extractor: topic.Fake{}, Images: imagegen.Placeholder{}, Panel: &panel.Server{},
		WindowSegments: 4, ExtractEvery: 4, Now: func() time.Time { return now }}
	for _, l := range lines[:4] {
		if err := feed(p, l); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(p.Mem.Wall(now)); n != 1 {
		t.Fatalf("wall has %d pictures, want 1", n)
	}
	four := p.Draft(set, false, collage.Look{}, collage.DefaultSize)
	set.MaxTiles = 1
	one := p.Draft(set, false, collage.Look{}, collage.DefaultSize)
	if bytes.Equal(four.Pix, one.Pix) {
		t.Fatal("four places and one place give the same preview")
	}
	if n := len(p.Mem.Wall(now)); n != 1 {
		t.Fatalf("after the preview the wall has %d pictures, want 1", n)
	}
}

type flaky struct{ down *bool }

func (f flaky) Generate(ctx context.Context, prompt string) (image.Image, error) {
	if *f.down {
		return nil, errors.New("image service is down")
	}
	return imagegen.Placeholder{}.Generate(ctx, prompt)
}

func TestFailedPictureIsTriedAgain(t *testing.T) {
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	down := true
	p := &Pipeline{
		Mem: memory.New(memory.Settings{HalfLife: 48 * time.Hour, MinConfidence: 0.5, MinMentions: 1,
			DailyImageCap: 10, MaxTiles: 9, MinWeight: 0.1, BlockFor: time.Hour}),
		Extractor: said{Topic: "garden gnome theft", Confidence: 0.9, VisualPrompt: "a gnome", Sensitive: "none"},
		Images:    flaky{&down}, Panel: &panel.Server{}, WindowSegments: 1, ExtractEvery: 1, Now: func() time.Time { return now },
	}
	if err := feed(p, "synthetic talk"); err != nil {
		t.Fatal(err)
	}
	p.Tick()
	if n := p.ImagesToday(); n != 0 || len(p.Wall()) != 0 || len(p.Waiting()) != 0 {
		t.Fatalf("after a failed picture: %d counted, %d on the wall, %d waiting", n, len(p.Wall()), len(p.Waiting()))
	}
	down = false
	if err := feed(p, "synthetic talk"); err != nil {
		t.Fatal(err)
	}
	if w := p.Wall(); len(w) != 1 || p.Picture(w[0].Label) == nil || p.ImagesToday() != 1 {
		t.Fatalf("after the service is back: wall %+v, %d counted", w, p.ImagesToday())
	}
}

func TestRepeatMentionKeepsThePage(t *testing.T) {
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	srv := &panel.Server{}
	p := &Pipeline{
		Mem: memory.New(memory.Settings{HalfLife: 48 * time.Hour, MinConfidence: 0.5, MinMentions: 1,
			DailyImageCap: 10, MaxTiles: 9, MinWeight: 0.1, BlockFor: time.Hour}),
		Extractor: said{Topic: "garden gnome theft", Confidence: 0.9, VisualPrompt: "a gnome", Sensitive: "none"},
		Images:    imagegen.Placeholder{}, Panel: srv, WindowSegments: 1, ExtractEvery: 1, Now: func() time.Time { return now },
	}
	if err := feed(p, "synthetic talk"); err != nil {
		t.Fatal(err)
	}
	page := srv.ETag()
	now = now.Add(7 * time.Minute)
	if err := feed(p, "synthetic talk"); err != nil {
		t.Fatal(err)
	}
	if srv.ETag() != page {
		t.Fatal("a repeat mention that moved no tile made a new page")
	}
	if w := p.Wall(); w[0].Mentions != 2 {
		t.Fatalf("mentions: %d", w[0].Mentions)
	}
}

// slow is an image provider that waits until it is let go.
type slow struct{ go_ chan struct{} }

func (s slow) Generate(ctx context.Context, prompt string) (image.Image, error) {
	<-s.go_
	return imagegen.Placeholder{}.Generate(ctx, prompt)
}

// A slow picture does not hold up topic detection, and its topic stays off
// the frame and out of the queue until the picture is there.
func TestPictureDoesNotBlockTopics(t *testing.T) {
	images := slow{make(chan struct{})}
	p := &Pipeline{
		Mem:       memory.New(memory.Settings{HalfLife: time.Hour, MinConfidence: 0.5, MinMentions: 1, DailyImageCap: 10, MaxTiles: 9, MinWeight: 0.1}),
		Extractor: fixed{Topic: "garden gnome theft", Confidence: 0.9, VisualPrompt: "a gnome", Sensitive: "none"},
		Images:    images, Panel: &panel.Server{}, WindowSegments: 1, ExtractEvery: 1, Now: time.Now,
	}
	for range 3 {
		if err := p.Feed(context.Background(), "synthetic talk"); err != nil {
			t.Fatal(err)
		}
	}
	p.Tick()
	if n, q := len(p.Wall()), len(p.Waiting()); n != 0 || q != 0 {
		t.Fatalf("before the picture: %d on the wall, %d waiting", n, q)
	}
	close(images.go_)
	p.Wait()
	if w := p.Wall(); len(w) != 1 || w[0].Mentions != 3 || p.Picture(w[0].Label) == nil {
		t.Fatalf("after the picture: %+v", w)
	}
}

// counting draws a new placeholder each call, so a redraw can be told apart.
type counting struct{ n *int }

func (c counting) Generate(ctx context.Context, _ string) (image.Image, error) {
	*c.n++
	return imagegen.Placeholder{}.Generate(ctx, fmt.Sprint(*c.n))
}

func TestRedraw(t *testing.T) {
	now := time.Date(2026, 11, 8, 20, 0, 0, 0, time.UTC)
	calls := 0
	p := &Pipeline{
		Mem: memory.New(memory.Settings{HalfLife: 48 * time.Hour, MinConfidence: 0.5, MinMentions: 1,
			DailyImageCap: 2, MaxTiles: 9, MinWeight: 0.1, BlockFor: 24 * time.Hour}),
		Extractor: fixed{Topic: "garden gnome theft", Confidence: 0.9, VisualPrompt: "a gnome", Sensitive: "none"},
		Images:    counting{&calls}, Panel: &panel.Server{}, Style: "Comic",
		WindowSegments: 1, ExtractEvery: 1, Now: func() time.Time { return now },
	}
	if err := feed(p, "synthetic talk"); err != nil {
		t.Fatal(err)
	}
	first, page := p.Picture("garden gnome theft"), p.Panel.ETag()
	now = now.Add(time.Minute)
	p.Style = "Woodcut"
	if err := p.Redraw(context.Background(), "garden gnome theft"); err != nil {
		t.Fatal(err)
	}
	if p.Picture("garden gnome theft") == first || p.Panel.ETag() == page {
		t.Fatal("redraw kept the old picture or page")
	}
	w := p.Wall()
	if w[0].Style != "Woodcut" || !w[0].Made.Equal(now) || p.ImagesToday() != 2 {
		t.Fatalf("after redraw %+v, %d today; want new style, made now, 2 today", w[0], p.ImagesToday())
	}
	if err := p.Redraw(context.Background(), "garden gnome theft"); err == nil || calls != 2 {
		t.Fatalf("over the cap: %v, %d calls; want refused before a call", err, calls)
	}
	if err := p.Redraw(context.Background(), "no such topic"); !errors.Is(err, ErrGone) {
		t.Fatalf("unknown topic: %v", err)
	}
}

// With rotation on, Tick redraws the frame when the next set of pictures is
// due, without any new topic.
func TestTickRotates(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	p := &Pipeline{
		Mem: memory.New(memory.Settings{HalfLife: time.Hour, MinConfidence: 0.5, MinMentions: 1,
			DailyImageCap: 10, MaxTiles: 1, MinWeight: 0.1, Newest: true, RotateEvery: time.Hour}),
		Images: imagegen.Placeholder{}, Panel: &panel.Server{},
		WindowSegments: 1, ExtractEvery: 1, Now: func() time.Time { return now },
	}
	for _, label := range []string{"garden gnome theft", "rollerblading date"} {
		p.Extractor = fixed{Topic: label, Confidence: 0.9, VisualPrompt: "a picture", Sensitive: "none"}
		if err := feed(p, "synthetic talk"); err != nil {
			t.Fatal(err)
		}
	}
	p.Tick()
	before, tag := p.Wall()[0].Label, p.Panel.ETag()
	now = now.Add(time.Hour)
	p.Tick()
	if after := p.Wall()[0].Label; after == before || p.Panel.ETag() == tag {
		t.Fatalf("an hour later the frame still shows %q (etag changed: %v)", after, p.Panel.ETag() != tag)
	}
}
