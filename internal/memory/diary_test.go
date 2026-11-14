package memory

import (
	"testing"
	"time"
)

func diary() Settings {
	s := settings()
	s.Diary, s.DiaryAt, s.MinMentions, s.MaxTiles, s.DailyImageCap = true, 7*60, 2, 2, 10
	return s
}

func TestDiaryCollectsAndPaintsTheTopOnesInTheMorning(t *testing.T) {
	m := New(diary())
	evening := t0 // 18:00
	if got := m.Edition(evening); got != nil {
		t.Fatalf("first call starts the clock, got %v", got)
	}
	// Synthetic topics: three talked about twice or more, one only once.
	for i, l := range []string{"ski trip", "ski trip", "ski trip", "cat", "cat", "garden", "garden", "boat"} {
		if d := m.Observe(l, l+" picture", 0.9, evening.Add(time.Duration(i)*time.Minute)); d != Pending {
			t.Fatalf("%s: %s, want pending (no pictures during the day)", l, d)
		}
	}
	if m.ImagesToday(evening) != 0 || len(m.Waiting()) != 0 {
		t.Fatal("diary mode made a picture before the morning")
	}
	if n, next := m.NextEdition(evening); n != 3 || !next.Equal(time.Date(2026, 11, 9, 7, 0, 0, 0, time.UTC)) {
		t.Fatalf("next edition: %d at %s", n, next)
	}
	if got := m.Edition(evening.Add(12*time.Hour + 59*time.Minute)); got != nil { // 06:59
		t.Fatalf("before 07:00: %v", got)
	}
	morning := time.Date(2026, 11, 9, 7, 0, 30, 0, time.UTC)
	got := m.Edition(morning)
	if len(got) != 2 || got[0].Label != "ski trip" {
		t.Fatalf("edition %v, want ski trip first and two topics (MaxTiles)", got)
	}
	if again := m.Edition(morning.Add(time.Minute)); again != nil {
		t.Fatalf("second pick the same morning: %v", again)
	}
	for _, p := range got {
		if !m.Made(p.Label, morning) {
			t.Fatalf("Made(%q) false", p.Label)
		}
	}
	m.Reveal(morning)
	if w := m.Wall(morning); len(w) != 2 {
		t.Fatalf("wall %v, want both up at once", w)
	}
	if m.ImagesToday(morning) != 2 {
		t.Fatalf("images today %d, want 2", m.ImagesToday(morning))
	}
	// A topic on the wall that comes up again is bumped, not painted again.
	if d := m.Observe("ski trip", "", 0.9, morning.Add(time.Hour)); d != Bump {
		t.Fatalf("repeat of a painted topic: %s", d)
	}
}

func TestDiaryRespectsTheDailyLimit(t *testing.T) {
	s := diary()
	s.DailyImageCap = 1
	m := New(s)
	m.Edition(t0)
	for _, l := range []string{"ski trip", "ski trip", "cat", "cat"} {
		m.Observe(l, "", 0.9, t0)
	}
	if got := m.Edition(time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)); len(got) != 1 {
		t.Fatalf("edition %v, want 1 topic within the limit", got)
	}
}

func TestDiaryCollectedTopicsAreKept(t *testing.T) {
	m := New(diary())
	m.Observe("ski trip", "skis", 0.9, t0)
	k := m.Keep()
	if len(k) != 1 || !k[0].Collected {
		t.Fatalf("keep %+v, want the collected topic", k)
	}
	back := New(diary())
	back.Restore(k)
	if d := back.Observe("ski trip", "skis", 0.9, t0.Add(time.Minute)); d != Pending {
		t.Fatalf("after restore: %s", d)
	}
	if n, _ := back.NextEdition(t0); n != 1 {
		t.Fatalf("collected after restore: %d", n)
	}
}

func TestDiarySkipsTopicsThatFaded(t *testing.T) {
	s := diary()
	s.HalfLife = time.Hour
	m := New(s)
	m.Edition(t0)
	m.Observe("cat", "", 0.9, t0)
	m.Observe("cat", "", 0.9, t0)
	if got := m.Edition(time.Date(2026, 11, 9, 7, 0, 0, 0, time.UTC)); len(got) != 0 {
		t.Fatalf("edition %v, want nothing: it faded overnight", got)
	}
}
