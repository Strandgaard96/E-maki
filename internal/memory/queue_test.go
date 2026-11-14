package memory

import (
	"testing"
	"time"
)

func TestWaitingMoveUpAndShowNow(t *testing.T) {
	s := Settings{HalfLife: time.Hour, MinConfidence: 0.5, MinMentions: 1, DailyImageCap: 10, MaxTiles: 9, MinWeight: 0.1,
		BlockFor: time.Hour, Every: time.Hour}
	m := New(s)
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	for i, l := range []string{"garden gnome theft", "burnt lasagne", "ski trip mishap"} {
		m.Observe(l, "a picture of "+l, 0.9, now.Add(time.Duration(i)*time.Minute))
	}
	m.Reveal(now) // the first goes up, the others wait an hour
	names := func() (out []string) {
		for _, t := range m.Waiting() {
			out = append(out, t.Label)
		}
		return out
	}
	if got := names(); len(got) != 2 || got[0] != "burnt lasagne" {
		t.Fatalf("waiting: %v", got)
	}
	if !m.MoveUp("ski trip mishap") || names()[0] != "ski trip mishap" {
		t.Fatalf("after move up: %v", names())
	}
	if m.MoveUp("garden gnome theft") {
		t.Fatal("a topic already up cannot move in the queue")
	}
	later := now.Add(5 * time.Minute)
	if !m.ShowNow("burnt lasagne", later) || len(m.Waiting()) != 1 {
		t.Fatalf("show now: %v", names())
	}
	if _, next := m.Queue(later); !next.Equal(later.Add(time.Hour)) {
		t.Fatalf("the wait starts again from a picture put up by hand: next at %v", next)
	}
	if w := m.Waiting(); w[0].Made.IsZero() || w[0].Prompt == "" {
		t.Fatalf("a waiting topic keeps where it came from: %+v", w[0])
	}
}
