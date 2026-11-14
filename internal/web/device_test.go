package web

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"net/http"
	"testing"

	"github.com/Strandgaard96/e-maki/internal/config"
)

// get fetches a path the way a frame does: a bare GET, no cookies, no
// browser headers, optionally with the ETag it saw last.
func get(t *testing.T, url, etag string) (*http.Response, []byte) {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, body
}

// A device that only fetches the picture and has no buttons, like an ESP32
// frame, a TV or a browser: every format works with the default settings
// and nothing waits for a press.
func TestDeviceWithoutButtons(t *testing.T) {
	ts, w, _ := setup(t, "")
	if n := len(w.snapshot().Settings.PanelButtons); n != 0 {
		t.Fatalf("default settings have %d buttons, want none", n)
	}
	for _, f := range []struct {
		path  string
		check func([]byte) error
	}{
		{"/frame.png", func(b []byte) error {
			img, err := png.Decode(bytes.NewReader(b))
			if err != nil {
				return err
			}
			if p, ok := img.(*image.Paletted); !ok || len(p.Palette) != 6 || img.Bounds().Size() != image.Pt(1600, 1200) {
				t.Errorf("frame.png is a %T of %v, want a 6-colour 1600x1200 paletted PNG", img, img.Bounds().Size())
			}
			return nil
		}},
		{"/frame.e6", func(b []byte) error {
			if len(b) != 20+1600*1200/2 || string(b[:4]) != "E6F1" {
				t.Errorf("frame.e6: %d bytes starting %q, want E6F1 and %d bytes", len(b), b[:min(4, len(b))], 20+1600*1200/2)
			}
			return nil
		}},
	} {
		resp, body := get(t, ts.URL+f.path, "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: %s", f.path, resp.Status)
		}
		if err := f.check(body); err != nil {
			t.Fatalf("%s: %v", f.path, err)
		}
		// Polling again with the ETag costs the device nothing.
		if again, _ := get(t, ts.URL+f.path, resp.Header.Get("ETag")); again.StatusCode != http.StatusNotModified {
			t.Errorf("%s with its ETag: %s, want 304", f.path, again.Status)
		}
	}
	if w.snapshot().Settings.Paused || len(w.Pipeline.Wall()) != 1 {
		t.Fatal("fetching changed something")
	}
	// A stray press on a frame without buttons is answered and does nothing.
	if resp := send(t, "POST", ts.URL+"/press", "application/x-www-form-urlencoded", "button=A", nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("press: %s", resp.Status)
	}
	if w.snapshot().Settings.Paused || len(w.Pipeline.Wall()) != 1 {
		t.Fatal("a press on a frame without buttons changed something")
	}
}

// A device with buttons sends one POST /press per press, as the Inky client
// does, and each letter does what panel_buttons sets.
func TestDeviceWithButtons(t *testing.T) {
	ts, w, saved := setup(t, "")
	w.SetSettings(config.Settings{Mode: "party", PanelButtons: map[string]string{"A": "pause", "B": "remove", "C": "pin", "D": ""}})
	press := func(b string) {
		t.Helper()
		if resp := send(t, "POST", ts.URL+"/press", "application/x-www-form-urlencoded", "button="+b, nil); resp.StatusCode != http.StatusNoContent {
			t.Fatalf("press %s: %s", b, resp.Status)
		}
	}
	for _, want := range []bool{true, false} {
		press("A")
		if w.snapshot().Settings.Paused != want || (*saved)[len(*saved)-1].Paused != want {
			t.Fatalf("after A: paused %v, want %v", w.snapshot().Settings.Paused, want)
		}
	}
	press("D") // set to nothing
	press("E") // a button the frame doesn't have
	if w.snapshot().Settings.Paused || len(w.Pipeline.Wall()) != 1 {
		t.Fatal("D or E did something")
	}
	press("C")
	if wall := w.Pipeline.Wall(); len(wall) != 1 || !wall[0].Pinned {
		t.Fatalf("C did not pin: %+v", wall)
	}
	press("B")
	if wall := w.Pipeline.Wall(); len(wall) != 0 {
		t.Fatalf("B did not remove the picture: %+v", wall)
	}
	if resp, _ := get(t, ts.URL+"/frame.png", ""); resp.StatusCode != http.StatusOK {
		t.Fatalf("frame.png after the presses: %s", resp.Status)
	}
}
