package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Strandgaard96/e-maki/internal/imagegen"
	"github.com/Strandgaard96/e-maki/internal/memory"
	"github.com/Strandgaard96/e-maki/internal/panel"
	"github.com/Strandgaard96/e-maki/internal/topic"
)

type said topic.Result

func (s said) Extract(context.Context, string, []string) (topic.Result, error) {
	return topic.Result(s), nil
}

func keeper(dir string) *Pipeline {
	return &Pipeline{
		Mem:       memory.New(memory.Settings{HalfLife: time.Hour, MinConfidence: 0.5, MinMentions: 1, DailyImageCap: 10, MaxTiles: 9, MinWeight: 0.1, BlockFor: time.Hour}),
		Extractor: said{Topic: "garden gnome theft", Confidence: 0.9, VisualPrompt: "a gnome", Sensitive: "none"},
		Images:    imagegen.Placeholder{}, Panel: &panel.Server{}, WindowSegments: 1, ExtractEvery: 1,
		Style: "Comic", KeepDir: dir, Now: time.Now,
	}
}

func TestKeptPicturesComeBackAfterARestart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "pictures")
	p := keeper(dir)
	if err := feed(p, "synthetic talk"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, keepIndex))
	if err != nil {
		t.Fatal(err)
	}
	// The kept index holds the topic and the picture description, never the transcript.
	if s := string(b); !strings.Contains(s, "garden gnome theft") || !strings.Contains(s, "a gnome") || strings.Contains(s, "synthetic talk") {
		t.Fatalf("index: %s", s)
	}
	again := keeper(dir)
	if err := again.Restore(); err != nil {
		t.Fatal(err)
	}
	if err := again.Start(); err != nil {
		t.Fatal(err)
	}
	w := again.Wall()
	if len(w) != 1 || w[0].Label != "garden gnome theft" || w[0].Style != "Comic" || again.Picture(w[0].Label) == nil {
		t.Fatalf("wall after restart: %+v", w)
	}
	// Turning keeping off deletes what was kept.
	if err := again.Configure(Options{Extractor: again.Extractor, WindowSegments: 1, ExtractEvery: 1, Images: again.Images}, again.Mem.Settings()); err != nil {
		t.Fatal(err)
	}
	if left, _ := os.ReadDir(dir); len(left) != 0 {
		t.Fatalf("files left after turning keeping off: %v", left)
	}
}

func TestDailyCountAndRemovedTopicsSurviveARestart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "pictures")
	p := keeper(dir)
	if err := feed(p, "synthetic talk"); err != nil {
		t.Fatal(err)
	}
	if !p.Remove("garden gnome theft") {
		t.Fatal("not on the wall")
	}
	again := keeper(dir)
	if err := again.Restore(); err != nil {
		t.Fatal(err)
	}
	if n := again.ImagesToday(); n != 1 {
		t.Fatalf("pictures made today after a restart: %d", n)
	}
	if err := feed(again, "synthetic talk"); err != nil {
		t.Fatal(err)
	}
	if w := again.Wall(); len(w) != 0 || again.ImagesToday() != 1 {
		t.Fatalf("a removed topic came back after a restart: %+v", w)
	}
}

func TestDiaryPaintsInTheMorningAndKeepsWhatItCollected(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "pictures")
	now := time.Date(2026, 11, 8, 20, 0, 0, 0, time.UTC)
	p := keeper(dir)
	p.Now = func() time.Time { return now }
	s := p.Mem.Settings()
	s.Diary, s.DiaryAt, s.MinMentions, s.HalfLife = true, 7*60, 2, 48*time.Hour
	p.Mem.SetSettings(s)
	p.Edition(context.Background()) // starts the clock
	for range 2 {
		if err := feed(p, "synthetic talk"); err != nil {
			t.Fatal(err)
		}
	}
	if len(p.Wall()) != 0 || p.ImagesToday() != 0 {
		t.Fatal("diary mode painted during the day")
	}
	// A restart overnight keeps the collected topic, not the talk.
	again := keeper(dir)
	again.Now = p.Now
	again.Mem.SetSettings(s)
	if err := again.Restore(); err != nil {
		t.Fatal(err)
	}
	again.Edition(context.Background())
	now = time.Date(2026, 11, 9, 7, 1, 0, 0, time.UTC)
	again.Edition(context.Background())
	w := again.Wall()
	if len(w) != 1 || w[0].Label != "garden gnome theft" || again.Picture(w[0].Label) == nil {
		t.Fatalf("wall after the morning: %+v", w)
	}
}
