// Package health keeps track of whether the sidecars and the image
// provider answer, for the settings page. It holds states, timings and
// error messages only, never audio, transcripts or pictures.
package health

import (
	"context"
	"image"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Strandgaard96/e-maki/internal/imagegen"
	"github.com/Strandgaard96/e-maki/internal/stt"
	"github.com/Strandgaard96/e-maki/internal/topic"
)

// The services the page lists.
const (
	Speech = "speech"
	Topics = "topics"
	Images = "images"
)

// State says how a service is doing.
type State string

const (
	Unknown State = "unknown" // not checked and not used yet
	Up      State = "up"      // answers
	Loading State = "loading" // answers, but is still loading its model
	Down    State = "down"    // does not answer
	Failing State = "failing" // answers, but its last call failed
)

// Service is one service as the page shows it.
type Service struct {
	State  State     `json:"state"`
	Error  string    `json:"error,omitempty"`  // why the last call failed, when it did
	LastAt time.Time `json:"last_at,omitzero"` // when it was last called
	Took   float64   `json:"took,omitempty"`   // seconds the last call took
}

type entry struct {
	probe   State // from the last probe; Unknown when not probed
	lastAt  time.Time
	took    time.Duration
	lastErr string
}

// Tracker records calls and probes. The zero value is ready; a nil
// Tracker records nothing.
type Tracker struct {
	mu sync.Mutex
	m  map[string]*entry
}

func (t *Tracker) get(name string) *entry {
	if t.m == nil {
		t.m = map[string]*entry{}
	}
	e, ok := t.m[name]
	if !ok {
		e = &entry{probe: Unknown}
		t.m[name] = e
	}
	return e
}

// Record notes a call that started at start and ended with err.
func (t *Tracker) Record(name string, start time.Time, err error) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	e := t.get(name)
	e.lastAt, e.took, e.lastErr = time.Now(), time.Since(start), ""
	if err != nil {
		e.lastErr = err.Error()
		if len(e.lastErr) > 300 {
			e.lastErr = e.lastErr[:300] + "…"
		}
	}
}

// setProbe stores a probe's result.
func (t *Tracker) setProbe(name string, s State) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.get(name).probe = s
}

// Forget drops what is known about a service, e.g. when another one is
// picked on the settings page.
func (t *Tracker) Forget(name string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	delete(t.m, name)
	t.mu.Unlock()
}

// Get returns the service as the page shows it.
func (t *Tracker) Get(name string) Service {
	if t == nil {
		return Service{State: Unknown}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	e := t.get(name)
	s := Service{State: e.probe, Error: e.lastErr, LastAt: e.lastAt, Took: e.took.Seconds()}
	switch {
	case e.probe == Down || e.probe == Loading:
	case e.lastErr != "":
		s.State = Failing
	case !e.lastAt.IsZero():
		s.State = Up
	}
	return s
}

// Probe asks a server whether it answers: 200 is up, 503 is still loading
// (whisper-server and llama-server say so while they load a model). Any
// other answer still counts as up; the calls themselves show what fails.
func Probe(ctx context.Context, c *http.Client, url string) (State, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Down, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return Down, err
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusServiceUnavailable {
		return Loading, nil
	}
	return Up, nil
}

// startGrace is how long Watch keeps quiet about a service that has not
// answered yet: at start the sidecars are often still starting, and their
// names do not even resolve until their containers exist.
const startGrace = 2 * time.Minute

// Watch probes the addresses targets returns every interval until ctx
// ends, and logs when a service starts or stops answering. targets maps a
// service name to its address; a service left out is not probed.
func (t *Tracker) Watch(ctx context.Context, every time.Duration, targets func() map[string]string) {
	c := &http.Client{Timeout: 3 * time.Second}
	start := time.Now()
	logged := map[string]State{} // what the log last said about each service
	for {
		for name, url := range targets() {
			s, err := Probe(ctx, c, url)
			t.setProbe(name, s)
			was, ok := logged[name]
			if s == was || !ok && s == Down && time.Since(start) < startGrace {
				continue
			}
			logged[name] = s
			switch s {
			case Up:
				log.Printf("%s: answering at %s", name, url)
			case Loading:
				log.Printf("%s: loading its model at %s", name, url)
			default:
				log.Printf("%s: not answering at %s: %v", name, url, err)
			}
		}
		select {
		case <-time.After(every):
		case <-ctx.Done():
			return
		}
	}
}

// Transcriber records each call of T as Speech.
type Transcriber struct {
	T       stt.Transcriber
	Tracker *Tracker
}

func (w Transcriber) Transcribe(ctx context.Context, pcm []int16, lang string) (stt.Segment, error) {
	start := time.Now()
	seg, err := w.T.Transcribe(ctx, pcm, lang)
	w.Tracker.Record(Speech, start, err)
	return seg, err
}

// Extractor records each call of E as Topics.
type Extractor struct {
	E       topic.Extractor
	Tracker *Tracker
	Name    string // the model and where it runs, for the log
}

func (w Extractor) String() string { return w.Name }

func (w Extractor) Extract(ctx context.Context, window string, recent []string) (topic.Result, error) {
	start := time.Now()
	r, err := w.E.Extract(ctx, window, recent)
	w.Tracker.Record(Topics, start, err)
	return r, err
}

// Provider records each call of P as Images.
type Provider struct {
	P       imagegen.Provider
	Tracker *Tracker
}

func (w Provider) Generate(ctx context.Context, prompt string) (image.Image, error) {
	start := time.Now()
	img, err := w.P.Generate(ctx, prompt)
	w.Tracker.Record(Images, start, err)
	return img, err
}
