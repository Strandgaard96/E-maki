package memory

import (
	"math"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC)

func settings() Settings {
	return Settings{HalfLife: 48 * time.Hour, MinConfidence: 0.5, MinMentions: 1, DailyImageCap: 10, MaxTiles: 9, MinWeight: 0.2, BlockFor: 7 * 24 * time.Hour}
}

func TestRepeatTopicBumpsWithoutNewImage(t *testing.T) {
	m := New(settings())
	var got []Decision
	for i, l := range []string{"ski trip", "ski trip", "cat", "Ski  Trip"} {
		got = append(got, m.Observe(l, "", 0.9, t0.Add(time.Duration(i)*time.Minute)))
	}
	want := []Decision{NewTopic, Bump, NewTopic, Bump}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("decisions %v, want %v", got, want)
		}
	}
	m.Reveal(t0)
	wall := m.Wall(t0.Add(time.Hour))
	if len(wall) != 2 || Key(wall[0].Label) != "ski trip" {
		t.Fatalf("wall %v, want ski trip first", wall)
	}
}

func TestSeenTwiceRule(t *testing.T) {
	s := settings()
	s.MinMentions = 2
	m := New(s)
	if d := m.Observe("hot dog", "", 0.9, t0); d != Pending {
		t.Fatalf("first sighting %s, want pending", d)
	}
	if d := m.Observe("hot dog", "", 0.9, t0.Add(time.Hour)); d != NewTopic {
		t.Fatalf("second sighting %s, want new", d)
	}
}

func TestLowConfidenceAndCap(t *testing.T) {
	s := settings()
	s.DailyImageCap = 1
	m := New(s)
	if d := m.Observe("tv show", "", 0.3, t0); d != LowConf {
		t.Fatalf("got %s, want low_conf", d)
	}
	m.Observe("a", "", 0.9, t0)
	if d := m.Observe("b", "", 0.9, t0); d != Capped {
		t.Fatalf("got %s, want capped", d)
	}
	if d := m.Observe("b", "", 0.9, t0.Add(24*time.Hour)); d != NewTopic {
		t.Fatalf("next day got %s, want new", d)
	}
}

func TestDecayAndPin(t *testing.T) {
	m := New(settings())
	m.Observe("cat", "", 1, t0)
	m.Observe("dog", "", 1, t0)
	m.Reveal(t0)
	m.Pin("dog", true)
	tp := m.topics["cat"]
	if w := tp.Weight(t0.Add(48*time.Hour), 48*time.Hour); math.Abs(w-0.5) > 1e-9 {
		t.Fatalf("weight after one half-life %v, want 0.5", w)
	}
	wall := m.Wall(t0.Add(30 * 24 * time.Hour))
	if len(wall) != 1 || wall[0].Label != "dog" {
		t.Fatalf("after 30 days wall %v, want only the pinned dog", wall)
	}
}

func TestRemoveBlocksTopic(t *testing.T) {
	m := New(settings())
	m.Observe("doctor appointment", "", 0.9, t0)
	m.Remove("doctor appointment", t0)
	if len(m.Wall(t0)) != 0 {
		t.Fatal("removed topic still on the wall")
	}
	if d := m.Observe("Doctor appointment", "", 0.9, t0.Add(24*time.Hour)); d != Blocked {
		t.Fatalf("got %s, want blocked", d)
	}
	if d := m.Observe("doctor appointment", "", 0.9, t0.Add(8*24*time.Hour)); d != NewTopic {
		t.Fatalf("after block got %s, want new", d)
	}
}

func TestRephrasedTopicIsSameTopic(t *testing.T) {
	m := New(settings())
	if d := m.Observe("online dating awkwardness", "", 0.9, t0); d != NewTopic {
		t.Fatalf("first: %s", d)
	}
	for _, l := range []string{"Online dating disaster", "awkward online dating apps"} {
		if d := m.Observe(l, "", 0.9, t0); d != Bump {
			t.Errorf("%q: %s, want bump", l, d)
		}
	}
	if c := m.Canonical("online dating disaster"); c != "online dating awkwardness" {
		t.Errorf("canonical %q", c)
	}
	for _, l := range []string{"online shopping", "dating", "ski trip to Lofoten"} {
		if d := m.Observe(l, "", 0.9, t0); d != NewTopic {
			t.Errorf("%q: %s, want new", l, d)
		}
	}
	m.Remove("ski trip to Lofoten", t0)
	if d := m.Observe("Lofoten ski holiday", "", 0.9, t0.Add(time.Hour)); d != Blocked {
		t.Errorf("rephrased removed topic: %s, want blocked", d)
	}
}

// Plurals and -ing/-ed forms count as the same word.
func TestSimilarStems(t *testing.T) {
	for _, c := range [][2]string{
		{"rollerblading date", "date on rollerblades"},
		{"burned lasagne for the in-laws", "burning lasagnes"},
		{"birthday parties for cats", "cat birthday party"},
	} {
		if !similar(c[0], c[1]) {
			t.Errorf("%q and %q should match", c[0], c[1])
		}
	}
	if similar("ski trip to Lofoten", "road trip to Skagen") {
		t.Error("different trips matched")
	}
}

// With Every set, new topics wait their turn; with Newest the wall is a
// rolling window that ignores fading.
func TestQueueAndRolling(t *testing.T) {
	s := settings()
	s.Every, s.Newest, s.MaxTiles = 10*time.Minute, true, 2
	m := New(s)
	for i, l := range []string{"cat", "dog", "owl"} {
		m.Observe(l, "", 0.9, t0.Add(time.Duration(i)*time.Second))
	}
	if !m.Reveal(t0) || m.Reveal(t0.Add(5*time.Minute)) {
		t.Fatal("one topic goes up at once, the next waits 10 minutes")
	}
	if n, next := m.Queue(t0.Add(5 * time.Minute)); n != 2 || !next.Equal(t0.Add(10*time.Minute)) {
		t.Fatalf("queue %d, next %v", n, next)
	}
	m.Reveal(t0.Add(10 * time.Minute))
	m.Reveal(t0.Add(20 * time.Minute))
	later := t0.Add(60 * 24 * time.Hour) // long faded
	m.Prune(later)
	wall := m.Wall(later)
	if len(wall) != 2 || wall[0].Label != "owl" || wall[1].Label != "dog" {
		t.Fatalf("rolling wall %v, want owl then dog", wall)
	}
}

func TestQuietHoursNewestBigAndClear(t *testing.T) {
	s := settings()
	s.QuietFrom, s.QuietTo, s.NewestBig = 22*60, 7*60, true
	m := New(s)
	night := time.Date(2026, 11, 8, 23, 30, 0, 0, time.UTC)
	m.Observe("cat", "", 0.9, night.Add(-2*time.Hour))
	m.Reveal(night.Add(-2 * time.Hour)) // 21:30, before the quiet hours
	m.Observe("dog", "", 0.9, night)
	m.Observe("dog", "", 0.9, night)
	if m.Reveal(night) {
		t.Fatal("nothing goes up in the quiet hours")
	}
	if _, next := m.Queue(night); !next.Equal(time.Date(2026, 11, 9, 7, 0, 0, 0, time.UTC)) {
		t.Fatalf("next %v, want 07:00 tomorrow", next)
	}
	morning := time.Date(2026, 11, 9, 7, 0, 0, 0, time.UTC)
	if !m.Reveal(morning) {
		t.Fatal("the waiting topic goes up when the quiet hours end")
	}
	if w := m.Wall(morning); len(w) != 2 || w[0].Label != "dog" {
		t.Fatalf("newest big: wall %v, want dog first though cat... is lighter", w)
	}
	m.Pin("cat", true)
	m.Clear()
	if w := m.Wall(morning); len(w) != 1 || w[0].Label != "cat" {
		t.Fatalf("after clear %v, want only the pinned cat", w)
	}
}

// A renamed topic keeps its picture and history, and the old label still
// leads to it, so it does not get a second picture.
func TestRename(t *testing.T) {
	m := New(settings())
	m.Observe("rollerblading date", "", 0.9, t0)
	m.Observe("ski trip to Lofoten", "", 0.9, t0)
	if m.Rename("rollerblading date", "ski trip to lofoten") {
		t.Error("renamed onto another topic")
	}
	if !m.Rename("rollerblading date", "Kim's skate fiasco") {
		t.Fatal("rename refused")
	}
	if d := m.Observe("rollerblading date", "", 0.9, t0); d != Bump {
		t.Errorf("old label after rename: %s, want bump", d)
	}
	m.Remove("Kim's skate fiasco", t0)
	if r := m.Removed(t0); len(r) != 1 || r[0].Label != "kim's skate fiasco" {
		t.Fatalf("removed %+v", r)
	}
	if !m.Allow("Kim's skate fiasco") || len(m.Removed(t0)) != 0 {
		t.Fatal("allow")
	}
	if d := m.Observe("Kim's skate fiasco", "", 0.9, t0); d != NewTopic {
		t.Errorf("allowed topic: %s, want new", d)
	}
}
