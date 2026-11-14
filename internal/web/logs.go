package web

import (
	"strings"
	"sync"
	"time"
)

// LogLine is one line of E-kiru's log, numbered so the page can ask for
// what is new.
type LogLine struct {
	N    int       `json:"n"`
	At   time.Time `json:"at"`
	Text string    `json:"text"`
}

// LogRing keeps the last lines of E-kiru's own log in memory for the Logs
// tab. Nothing goes to disk. It holds what the log holds: levels, timings,
// topic labels and picture descriptions, never what was said (see
// PRIVACY.md). Set it as an extra log output.
type LogRing struct {
	mu    sync.Mutex
	size  int
	lines []LogLine
	n     int
	part  string
}

// NewLogRing keeps the last size lines.
func NewLogRing(size int) *LogRing { return &LogRing{size: size} }

func (r *LogRing) Write(b []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	parts := strings.Split(r.part+string(b), "\n")
	r.part = parts[len(parts)-1]
	for _, l := range parts[:len(parts)-1] {
		r.n++
		r.lines = append(r.lines, LogLine{N: r.n, At: time.Now(), Text: l})
	}
	if over := len(r.lines) - r.size; over > 0 {
		r.lines = append([]LogLine(nil), r.lines[over:]...)
	}
	return len(b), nil
}

// Since returns the lines after line number n, oldest first.
func (r *LogRing) Since(n int) []LogLine {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []LogLine{}
	for _, l := range r.lines {
		if l.N > n {
			out = append(out, l)
		}
	}
	return out
}
