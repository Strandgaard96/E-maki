package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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
