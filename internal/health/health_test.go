package health

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestStates(t *testing.T) {
	tr := &Tracker{}
	if s := tr.Get(Images); s.State != Unknown {
		t.Fatalf("new service %v, want unknown", s.State)
	}
	tr.Record(Images, time.Now(), nil)
	if s := tr.Get(Images); s.State != Up || s.LastAt.IsZero() {
		t.Fatalf("after a call %+v, want up", s)
	}
	tr.Record(Images, time.Now(), errors.New("API key not valid"))
	if s := tr.Get(Images); s.State != Failing || s.Error != "API key not valid" {
		t.Fatalf("after a failed call %+v, want failing with the error", s)
	}
	tr.setProbe(Images, Down)
	if s := tr.Get(Images); s.State != Down {
		t.Fatalf("probe down %v, want down", s.State)
	}
	tr.Forget(Images)
	if s := tr.Get(Images); s.State != Unknown || s.Error != "" {
		t.Fatalf("after Forget %+v, want unknown", s)
	}
	var none *Tracker
	none.Record(Speech, time.Now(), nil) // a nil Tracker records nothing
	if none.Get(Speech).State != Unknown {
		t.Fatal("nil tracker")
	}
}

func TestProbe(t *testing.T) {
	code := http.StatusOK
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }))
	c := &http.Client{Timeout: time.Second}
	for _, tc := range []struct {
		code int
		want State
	}{{200, Up}, {503, Loading}, {404, Up}} {
		code = tc.code
		if s, _ := Probe(context.Background(), c, ts.URL+"/health"); s != tc.want {
			t.Errorf("status %d: %v, want %v", tc.code, s, tc.want)
		}
	}
	ts.Close()
	if s, err := Probe(context.Background(), c, ts.URL+"/health"); s != Down || err == nil {
		t.Errorf("closed server: %v %v, want down", s, err)
	}
}

func TestWatchQuietAtStart(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex
	log.SetOutput(syncWriter{&mu, &buf})
	defer log.SetOutput(os.Stderr)
	logs := func() string { mu.Lock(); defer mu.Unlock(); return buf.String() }
	wait := func(want string) {
		t.Helper()
		for range 200 {
			if strings.Contains(logs(), want) {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatalf("log %q has no %q", logs(), want)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	var url atomic.Value
	url.Store("http://127.0.0.1:1/health") // nothing listens yet
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var tr Tracker
	go tr.Watch(ctx, 5*time.Millisecond, func() map[string]string { return map[string]string{Speech: url.Load().(string)} })

	time.Sleep(50 * time.Millisecond)
	if strings.Contains(logs(), "not answering") {
		t.Fatalf("logged a sidecar that is still starting: %q", logs())
	}
	url.Store(ts.URL + "/health")
	wait("speech: answering")
	ts.Close()
	wait("speech: not answering") // once it has answered, going away is news
}

type syncWriter struct {
	mu *sync.Mutex
	w  *bytes.Buffer
}

func (s syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}
