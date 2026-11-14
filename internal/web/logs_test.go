package web

import "testing"

func TestLogRingKeepsTheLastLines(t *testing.T) {
	r := NewLogRing(3)
	r.Write([]byte("one\ntwo\nthr"))
	r.Write([]byte("ee\nfour\n"))
	got := r.Since(0)
	if len(got) != 3 || got[0].Text != "two" || got[2].Text != "four" || got[2].N != 4 {
		t.Fatalf("ring: %+v", got)
	}
	if s := r.Since(3); len(s) != 1 || s[0].Text != "four" {
		t.Fatalf("since 3: %+v", s)
	}
}
