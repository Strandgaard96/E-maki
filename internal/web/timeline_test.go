package web

import (
	"encoding/json"
	"image"
	"net/http"
	"testing"
	"time"

	"github.com/Strandgaard96/e-maki/internal/timeline"
)

func TestTimeline(t *testing.T) {
	ts, w, saved := setup(t, "")
	store, _ := timeline.Open(t.TempDir(), timeline.Options{ThumbPx: 480, Quality: 90})
	w.Timeline = store
	e, err := store.Add("garden gnome theft", "a gnome", "Comic", time.Now(), image.NewRGBA(image.Rect(0, 0, 64, 48)))
	if err != nil {
		t.Fatal(err)
	}
	get := func() timelineState {
		resp, err := http.Get(ts.URL + "/api/timeline")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var st timelineState
		json.NewDecoder(resp.Body).Decode(&st)
		return st
	}
	if st := get(); !st.On || len(st.Pictures) != 1 || st.Pictures[0].Prompt != "a gnome" {
		t.Fatalf("state %+v", st)
	}
	for _, q := range []string{"", "?size=thumb"} {
		resp, err := http.Get(ts.URL + "/api/timeline/" + e.ID + q)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/jpeg" {
			t.Errorf("picture%s: %d %s", q, resp.StatusCode, resp.Header.Get("Content-Type"))
		}
	}
	if resp, _ := http.Get(ts.URL + "/api/timeline/..%2fsettings.json"); resp.StatusCode != 404 {
		t.Errorf("unlisted file: %d", resp.StatusCode)
	}
	if r := send(t, "POST", ts.URL+"/api/timeline/keep", "application/json", `{"on":false}`, nil); r.StatusCode != 200 {
		t.Fatalf("keep: %d", r.StatusCode)
	}
	if s := (*saved)[len(*saved)-1]; s.TimelineOn() {
		t.Error("switching off was not saved")
	}
	if st := get(); st.On || len(st.Pictures) != 1 {
		t.Errorf("off should keep the saved pictures: %+v", st)
	}
	if r := send(t, "POST", ts.URL+"/api/timeline/delete", "application/json", `{}`, nil); r.StatusCode != 200 {
		t.Fatalf("delete: %d", r.StatusCode)
	}
	if st := get(); len(st.Pictures) != 0 {
		t.Errorf("after delete: %+v", st)
	}
}
