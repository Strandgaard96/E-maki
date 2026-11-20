package panel

import (
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServerProtocol(t *testing.T) {
	var pressed string
	s := &Server{Poll: 5, OnPress: func(b string) { pressed = b }}
	mux := http.NewServeMux()
	s.Register(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	if resp, _ := http.Get(ts.URL + "/frame.png"); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("before first page: %s", resp.Status)
	}
	if err := s.SetPage(load(t, "testdata/dither_input.png")); err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(ts.URL + "/frame.png")
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if p, ok := img.(*image.Paletted); !ok || len(p.Palette) != 6 {
		t.Fatalf("frame.png is %T, want a 6-colour paletted image", img)
	}
	req, _ := http.NewRequest("GET", ts.URL+"/frame.png", nil)
	req.Header.Set("If-None-Match", resp.Header.Get("ETag"))
	if resp, _ := http.DefaultClient.Do(req); resp.StatusCode != http.StatusNotModified {
		t.Fatalf("same ETag: %s, want 304", resp.Status)
	}
	if resp, _ := http.Post(ts.URL+"/press", "application/x-www-form-urlencoded", strings.NewReader("button=B")); resp.StatusCode != http.StatusNoContent || pressed != "B" {
		t.Fatalf("press: %s, pressed %q", resp.Status, pressed)
	}
}

// With a token the protocol lives under /<token>/ and nowhere else.
func TestServerToken(t *testing.T) {
	pressed := ""
	s := &Server{Token: "s3cret", OnPress: func(b string) { pressed = b }}
	page := load(t, "testdata/dither_input.png")
	defer func(g image.Point) { twoControllers = g }(twoControllers)
	twoControllers = page.Bounds().Size()
	mux := http.NewServeMux()
	s.Register(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()
	if err := s.SetPage(page); err != nil {
		t.Fatal(err)
	}
	form := "application/x-www-form-urlencoded"
	if resp, _ := http.Get(ts.URL + "/frame.png"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("frame.png without the token: %s", resp.Status)
	}
	if resp, _ := http.Post(ts.URL+"/press", form, strings.NewReader("button=B")); resp.StatusCode != http.StatusNotFound || pressed != "" {
		t.Errorf("press without the token: %s, pressed %q", resp.Status, pressed)
	}
	if resp, _ := http.Get(ts.URL + "/s3cret/frame.e6"); resp.StatusCode != http.StatusOK {
		t.Errorf("frame.e6 with the token: %s", resp.Status)
	}
	if resp, _ := http.Post(ts.URL+"/s3cret/press", form, strings.NewReader("button=B")); resp.StatusCode != http.StatusNoContent || pressed != "B" {
		t.Errorf("press with the token: %s, pressed %q", resp.Status, pressed)
	}
}

// A web page on another site cannot press the frame's buttons.
func TestPressFromAnotherSiteIsRefused(t *testing.T) {
	pressed := 0
	s := &Server{OnPress: func(string) { pressed++ }}
	mux := http.NewServeMux()
	s.Register(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()
	for name, c := range map[string]struct {
		hdr  map[string]string
		want int
	}{
		"a frame":        {nil, http.StatusNoContent},
		"cross-site":     {map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusForbidden},
		"another origin": {map[string]string{"Origin": "http://evil.example"}, http.StatusForbidden},
		"its own page":   {map[string]string{"Origin": ts.URL, "Sec-Fetch-Site": "same-origin"}, http.StatusNoContent},
	} {
		req, _ := http.NewRequest("POST", ts.URL+"/press", strings.NewReader("button=A"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for k, v := range c.hdr {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != c.want {
			t.Errorf("%s: %s, want %d", name, resp.Status, c.want)
		}
	}
	if pressed != 2 {
		t.Fatalf("pressed %d times, want 2", pressed)
	}
}

// The page learns which frame fetched and what it pressed, from the
// User-Agent of Fugleramme's clients; a browser leaves no trace.
func TestSeen(t *testing.T) {
	s := &Server{}
	mux := http.NewServeMux()
	s.Register(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()
	if err := s.SetPage(load(t, "testdata/dither_input.png")); err != nil {
		t.Fatal(err)
	}
	do := func(method, path, ua, body string) {
		t.Helper()
		req, _ := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
		req.Header.Set("User-Agent", ua)
		if body != "" {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	do("GET", "/frame.png", "Mozilla/5.0", "")
	if got := s.Seen(); got.Client != "" || got.Fetched != nil {
		t.Fatalf("after a browser: %+v", got)
	}
	do("GET", "/frame.e6?battery=3.91", "ESP32HTTPClient", "")
	if got := s.Seen(); got.Client != "xiao" || got.Fetched == nil || got.Pressed != nil {
		t.Fatalf("after the XIAO fetched: %+v", got)
	}
	do("POST", "/press", "ESP32HTTPClient", "button=C")
	if got := s.Seen(); got.Button != "C" || got.Pressed == nil {
		t.Fatalf("after the XIAO pressed C: %+v", got)
	}
	do("GET", "/frame.png", "Python-urllib/3.11", "")
	if got := s.Seen(); got.Client != "inky" || got.Button != "C" {
		t.Fatalf("after the Inky fetched: %+v", got)
	}
}
