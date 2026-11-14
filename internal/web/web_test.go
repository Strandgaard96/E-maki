package web

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Strandgaard96/e-maki/internal/audio"
	"github.com/Strandgaard96/e-maki/internal/collage"
	"github.com/Strandgaard96/e-maki/internal/config"
	"github.com/Strandgaard96/e-maki/internal/health"
	"github.com/Strandgaard96/e-maki/internal/imagegen"
	"github.com/Strandgaard96/e-maki/internal/memory"
	"github.com/Strandgaard96/e-maki/internal/panel"
	"github.com/Strandgaard96/e-maki/internal/pipeline"
	"github.com/Strandgaard96/e-maki/internal/topic"
)

type fixed topic.Result

func (f fixed) Extract(context.Context, string, []string) (topic.Result, error) {
	return topic.Result(f), nil
}

func setup(t *testing.T, token string) (*httptest.Server, *Server, *[]config.Settings) {
	t.Helper()
	srv := &panel.Server{Token: token}
	p := &pipeline.Pipeline{
		Mem:       memory.New(memory.Settings{HalfLife: time.Hour, MinConfidence: 0.5, MinMentions: 1, DailyImageCap: 10, MaxTiles: 9, MinWeight: 0.1, BlockFor: time.Hour}),
		Extractor: fixed{Topic: "garden gnome theft", Confidence: 0.9, VisualPrompt: "a gnome", Sensitive: "none"},
		Images:    imagegen.Placeholder{}, Panel: srv, WindowSegments: 1, ExtractEvery: 1, Now: time.Now,
	}
	if err := p.Feed(context.Background(), "synthetic talk"); err != nil {
		t.Fatal(err)
	}
	p.Wait()
	var saved []config.Settings
	w := &Server{Token: token, Pipeline: p, Panel: srv,
		Apply: func(s config.Settings) (config.Settings, error) {
			if s.Mode != "diary" && s.Mode != "party" {
				return s, errors.New(`mode must be "diary" or "party"`)
			}
			return s, nil
		},
		Save: func(s config.Settings) error { saved = append(saved, s); return nil },
	}
	w.SetSettings(config.Settings{Mode: "party", HalfLifeHours: 48, MinConfidence: 0.5, DailyImageCap: 10})
	srv.OnPress = w.Press
	mux := http.NewServeMux()
	srv.Register(mux)
	w.Register(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, w, &saved
}

func send(t *testing.T, method, url, ctype, body string, hdr map[string]string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Content-Type", ctype)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

func TestPageAndState(t *testing.T) {
	ts, _, _ := setup(t, "")
	for path, ctype := range map[string]string{"/": "text/html", "/view": "text/html", "/ui/app.js": "text/javascript",
		"/ui/app.css": "text/css", "/preview.png": "image/png"} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), ctype) {
			t.Errorf("%s: %s, %s", path, resp.Status, resp.Header.Get("Content-Type"))
		}
	}
	resp, _ := http.Get(ts.URL + "/")
	if !strings.Contains(resp.Header.Get("Content-Security-Policy"), "frame-ancestors 'none'") || resp.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Error("the page needs its CSP and no-referrer headers")
	}
	resp, _ = http.Get(ts.URL + "/api/state")
	var st state
	json.NewDecoder(resp.Body).Decode(&st)
	resp.Body.Close()
	if len(st.Wall) != 1 || st.Wall[0].Label != "garden gnome theft" || st.Page == "" || st.Settings.Mode != "party" || len(st.Styles) == 0 {
		t.Fatalf("state %+v", st)
	}
}

func TestChangesOnlyFromThePage(t *testing.T) {
	ts, w, saved := setup(t, "")
	good := `{"mode":"diary","image_style":"","block_sensitive":["health"],"block_names":true,"half_life_hours":24,"min_confidence":0.6,"daily_image_cap":5,"panel_rotation":0,"panel_poll_minutes":5}`
	for name, c := range map[string]struct {
		ctype, body string
		hdr         map[string]string
		want        int
	}{
		"form post":     {"application/x-www-form-urlencoded", good, nil, http.StatusUnsupportedMediaType},
		"other site":    {"application/json", good, map[string]string{"Origin": "http://evil.example"}, http.StatusForbidden},
		"cross-site":    {"application/json", good, map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusForbidden},
		"unknown key":   {"application/json", `{"whisper_url":"http://x"}`, nil, http.StatusBadRequest},
		"bad value":     {"application/json", strings.Replace(good, "diary", "fiesta", 1), nil, http.StatusUnprocessableEntity},
		"from the page": {"application/json", good, map[string]string{"Origin": ts.URL, "Sec-Fetch-Site": "same-origin"}, http.StatusOK},
	} {
		if resp := send(t, "PUT", ts.URL+"/api/settings", c.ctype, c.body, c.hdr); resp.StatusCode != c.want {
			t.Errorf("%s: %s, want %d", name, resp.Status, c.want)
		}
	}
	if len(*saved) != 1 || (*saved)[0].Mode != "diary" || w.snapshot().Settings.DailyImageCap != 5 {
		t.Fatalf("saved %+v", *saved)
	}
}

func TestTiles(t *testing.T) {
	ts, w, _ := setup(t, "")
	js := "application/json"
	if resp := send(t, "POST", ts.URL+"/api/tiles/pin", js, `{"label":"garden gnome theft"}`, nil); resp.StatusCode != 200 || !w.Pipeline.Wall()[0].Pinned {
		t.Fatalf("pin: %s", resp.Status)
	}
	if resp := send(t, "POST", ts.URL+"/api/tiles/remove", js, `{"label":"no such topic"}`, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("remove unknown: %s", resp.Status)
	}
	if resp := send(t, "POST", ts.URL+"/api/tiles/remove", js, `{"label":"garden gnome theft"}`, nil); resp.StatusCode != 200 || len(w.Pipeline.Wall()) != 0 {
		t.Fatalf("remove: %s", resp.Status)
	}
	if resp := send(t, "POST", ts.URL+"/api/tiles/explode", js, `{"label":"x"}`, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown action: %s", resp.Status)
	}
}

// With PANEL_TOKEN the page lives under /<token>/ like the frame.
func TestToken(t *testing.T) {
	ts, _, _ := setup(t, "s3cret")
	for path, want := range map[string]int{"/": 404, "/api/state": 404, "/s3cret/": 200, "/s3cret/api/state": 200, "/s3cret/ui/app.js": 200} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Errorf("%s: %s, want %d", path, resp.Status, want)
		}
	}
	if resp := send(t, "POST", ts.URL+"/api/tiles/remove", "application/json", `{"label":"garden gnome theft"}`, nil); resp.StatusCode != 404 {
		t.Errorf("remove without the token: %s", resp.Status)
	}
}

func TestPauseAndClear(t *testing.T) {
	ts, w, saved := setup(t, "")
	js := "application/json"
	if resp := send(t, "POST", ts.URL+"/api/pause", js, `{"paused":true}`, nil); resp.StatusCode != 200 || !w.snapshot().Settings.Paused || !(*saved)[0].Paused {
		t.Fatalf("pause: %s", resp.Status)
	}
	if resp := send(t, "POST", ts.URL+"/api/clear", js, `{}`, nil); resp.StatusCode != 200 || len(w.Pipeline.Wall()) != 0 {
		t.Fatalf("clear: %s, wall %v", resp.Status, w.Pipeline.Wall())
	}
}

// Button A on the frame pauses and resumes like the switch on the page.
func TestButtonA(t *testing.T) {
	ts, w, saved := setup(t, "")
	form := "application/x-www-form-urlencoded"
	for _, want := range []bool{true, false} {
		if resp := send(t, "POST", ts.URL+"/press", form, "button=A", nil); resp.StatusCode != http.StatusNoContent {
			t.Fatalf("press: %s", resp.Status)
		}
		if w.snapshot().Settings.Paused != want || (*saved)[len(*saved)-1].Paused != want {
			t.Fatalf("after A: paused %v, want %v", w.snapshot().Settings.Paused, want)
		}
	}
	// panel_buttons moves pause to B and turns A off.
	w.SetSettings(config.Settings{Mode: "party", PanelButtons: map[string]string{"A": "", "B": "pause"}})
	send(t, "POST", ts.URL+"/press", form, "button=A", nil)
	if w.snapshot().Settings.Paused {
		t.Fatal("A paused although it is set to do nothing")
	}
	send(t, "POST", ts.URL+"/press", form, "button=B", nil)
	if !w.snapshot().Settings.Paused {
		t.Fatal("B did not pause")
	}
}

func TestPassword(t *testing.T) {
	ts, w, _ := setup(t, "")
	auth, _ := LoadAuth(t.TempDir() + "/password.json")
	w.Auth = auth
	js := "application/json"
	if resp := send(t, "POST", ts.URL+"/api/password", js, `{"password":"short"}`, nil); resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("short password: %s", resp.Status)
	}
	if resp := send(t, "POST", ts.URL+"/api/password", js, `{"password":"correct horse"}`, nil); resp.StatusCode != 200 {
		t.Fatalf("set password: %s", resp.Status)
	}
	if resp, _ := http.Get(ts.URL + "/api/state"); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("state without signing in: %s", resp.Status)
	}
	if resp, _ := http.Get(ts.URL + "/preview.png"); resp.StatusCode != 200 {
		t.Errorf("the picture stays open like /frame.png: %s", resp.Status)
	}
	if resp := send(t, "POST", ts.URL+"/api/clear", js, `{}`, nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("clear without signing in: %s", resp.Status)
	}
	if resp := send(t, "POST", ts.URL+"/api/login", js, `{"password":"wrong one"}`, nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("wrong password: %s", resp.Status)
	}
	resp := send(t, "POST", ts.URL+"/api/login", js, `{"password":"correct horse"}`, nil)
	cookies := resp.Cookies()
	if resp.StatusCode != http.StatusNoContent || len(cookies) != 1 || !cookies[0].HttpOnly {
		t.Fatalf("sign in: %s, cookies %v", resp.Status, cookies)
	}
	req, _ := http.NewRequest("GET", ts.URL+"/api/state", nil)
	req.AddCookie(cookies[0])
	if resp, _ := http.DefaultClient.Do(req); resp.StatusCode != 200 {
		t.Errorf("state when signed in: %s", resp.Status)
	}
	again, _ := LoadAuth(auth.Path)
	if !again.Enabled() {
		t.Error("the password is kept for the next start")
	}
	forged := &http.Cookie{Name: cookieName, Value: "99999999999.00"}
	req, _ = http.NewRequest("GET", ts.URL+"/api/state", nil)
	req.AddCookie(forged)
	if resp, _ := http.DefaultClient.Do(req); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("forged cookie: %s", resp.Status)
	}
}

func TestDraftPreviewLeavesTheFrameAlone(t *testing.T) {
	ts, w, saved := setup(t, "")
	var got config.Settings
	w.Draft = func(s config.Settings) (image.Image, error) {
		got = s
		return w.Pipeline.Draft(memory.Settings{HalfLife: time.Hour, MaxTiles: 1, MinWeight: 0.1, Newest: true}, s.HideCaptions, collage.DefaultSize), nil
	}
	before := w.Panel.ETag()
	r, err := http.Get(ts.URL + "/api/preview.png?layout=single&hide_captions=true&tiles=3")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		t.Fatalf("preview: %d", r.StatusCode)
	}
	if _, err := png.Decode(r.Body); err != nil {
		t.Fatal(err)
	}
	// The draft is the settings in force with the query over them.
	if got.Layout != "single" || !got.HideCaptions || got.Tiles != 3 || got.Mode != "party" {
		t.Fatalf("draft settings: %+v", got)
	}
	if w.Panel.ETag() != before || len(*saved) != 0 {
		t.Fatal("a draft preview changed the frame or saved settings")
	}
	if s := w.Pipeline.Mem.Settings(); s.MaxTiles != 9 || s.Newest {
		t.Fatalf("a draft preview left its settings behind: %+v", s)
	}
	if r, _ := http.Get(ts.URL + "/api/preview.png?tiles=many"); r.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("bad tiles: %d", r.StatusCode)
	}
}

func TestMicListsDevicesAndNeedsThePassword(t *testing.T) {
	ts, w, _ := setup(t, "")
	level := -32.0
	w.Mic = func() Mic {
		return Mic{Live: true, Devices: []audio.Device{{Name: "USB microphone"}, {Name: "Built-in", Default: true}}, Active: "USB microphone", Level: &level}
	}
	r, err := http.Get(ts.URL + "/api/mic")
	if err != nil {
		t.Fatal(err)
	}
	var m Mic
	json.NewDecoder(r.Body).Decode(&m)
	r.Body.Close()
	if len(m.Devices) != 2 || m.Active != "USB microphone" || m.Level == nil || !m.Devices[1].Default {
		t.Fatalf("mic: %+v", m)
	}
	auth, _ := LoadAuth("")
	w.Auth = auth
	if err := auth.Set("correct horse"); err != nil {
		t.Fatal(err)
	}
	if r, _ := http.Get(ts.URL + "/api/mic"); r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("mic without signing in: %d", r.StatusCode)
	}
}

func TestQueueAndLogs(t *testing.T) {
	ts, w, _ := setup(t, "")
	if r := send(t, "POST", ts.URL+"/api/queue/drop", "application/json", `{"label":"not waiting"}`, nil); r.StatusCode != http.StatusNotFound {
		t.Fatalf("drop of a picture that does not wait: %d", r.StatusCode)
	}
	if r := send(t, "POST", ts.URL+"/api/queue/shuffle", "application/json", `{"label":"x"}`, nil); r.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown queue action: %d", r.StatusCode)
	}
	w.Logs = NewLogRing(10)
	w.Logs.Write([]byte("wall: 1 tiles\n"))
	r, err := http.Get(ts.URL + "/api/logs?after=0")
	if err != nil {
		t.Fatal(err)
	}
	var lines []LogLine
	json.NewDecoder(r.Body).Decode(&lines)
	r.Body.Close()
	if len(lines) != 1 || lines[0].Text != "wall: 1 tiles" {
		t.Fatalf("logs: %+v", lines)
	}
	if r, _ := http.Get(ts.URL + "/api/picture.png?label=garden%20gnome%20theft"); r.StatusCode != 200 {
		t.Fatalf("picture of a tile: %d", r.StatusCode)
	}
}

func TestTestPictureAndRedraw(t *testing.T) {
	ts, w, _ := setup(t, "")
	w.Images = imagegen.FromEnv(func(string) string { return "" })
	w.Health = &health.Tracker{}
	post := func(path, body string) (*http.Response, map[string]any) {
		req, _ := http.NewRequest("POST", ts.URL+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out map[string]any
		json.NewDecoder(resp.Body).Decode(&out)
		return resp, out
	}
	resp, out := post("/api/test-picture", `{"provider": "placeholder", "style": ""}`)
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(out["image"].(string), "data:image/png;base64,") {
		t.Fatalf("test picture: %d %v", resp.StatusCode, out)
	}
	if s := w.Health.Get(health.Images); s.State != health.Up {
		t.Fatalf("a test of the provider in use counts for its status: %+v", s)
	}
	if resp, _ := post("/api/test-picture", `{"provider": "gemini"}`); resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("provider not set up: %d, want 422", resp.StatusCode)
	}
	if resp, out := post("/api/redraw", `{"label": "garden gnome theft"}`); resp.StatusCode != http.StatusOK || out["health"] == nil {
		t.Fatalf("redraw: %d %v", resp.StatusCode, out)
	}
	if resp, _ := post("/api/redraw", `{"label": "gone"}`); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("redraw of an unknown topic: %d, want 404", resp.StatusCode)
	}
}
