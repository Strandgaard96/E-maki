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
