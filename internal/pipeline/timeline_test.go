package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/Strandgaard96/e-maki/internal/timeline"
)

func TestNewPicturesGoToTheTimeline(t *testing.T) {
	p := keeper("")
	p.Timeline, _ = timeline.Open(t.TempDir(), timeline.Options{ThumbPx: 480, Quality: 90})
	for range 2 { // the second mention is a bump: no new picture
		if err := feed(p, "synthetic talk"); err != nil {
			t.Fatal(err)
		}
	}
	list := p.Timeline.List()
	if len(list) != 1 || list[0].Label != "garden gnome theft" || list[0].Prompt != "a gnome" || list[0].Style != "Comic" {
		t.Fatalf("timeline %+v", list)
	}
}

func TestDiaryPicturesGoToTheTimeline(t *testing.T) {
	now := time.Date(2026, 11, 8, 20, 0, 0, 0, time.UTC)
	p := keeper("")
	p.Now = func() time.Time { return now }
	p.Timeline, _ = timeline.Open(t.TempDir(), timeline.Options{ThumbPx: 480, Quality: 90})
	s := p.Mem.Settings()
	s.Diary, s.DiaryAt, s.MinMentions, s.HalfLife = true, 7*60, 1, 48*time.Hour
	p.Mem.SetSettings(s)
	p.Edition(context.Background()) // starts the clock
	if err := feed(p, "synthetic talk"); err != nil {
		t.Fatal(err)
	}
	if n := len(p.Timeline.List()); n != 0 {
		t.Fatalf("%d pictures before the morning", n)
	}
	now = time.Date(2026, 11, 9, 7, 1, 0, 0, time.UTC)
	p.Edition(context.Background())
	if list := p.Timeline.List(); len(list) != 1 || list[0].Label != "garden gnome theft" {
		t.Fatalf("timeline after the morning: %+v", list)
	}
}
