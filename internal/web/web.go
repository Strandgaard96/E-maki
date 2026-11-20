// Package web serves E-maki's settings page: a preview of the frame, the
// tiles on it (pin or remove them) and the settings that change what it
// shows. It never shows or stores a transcript: only the topic labels that
// are on the frame anyway.
package web

import (
	"bytes"
	"cmp"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Strandgaard96/e-maki/internal/audio"
	"github.com/Strandgaard96/e-maki/internal/collage"
	"github.com/Strandgaard96/e-maki/internal/config"
	"github.com/Strandgaard96/e-maki/internal/health"
	"github.com/Strandgaard96/e-maki/internal/imagegen"
	"github.com/Strandgaard96/e-maki/internal/memory"
	"github.com/Strandgaard96/e-maki/internal/panel"
	"github.com/Strandgaard96/e-maki/internal/pipeline"
	"github.com/Strandgaard96/e-maki/internal/stt"
	"github.com/Strandgaard96/e-maki/internal/timeline"
	"github.com/Strandgaard96/e-maki/internal/topic"
	xdraw "golang.org/x/image/draw"
)

//go:embed static
var static embed.FS

// Status is what the page says about the running setup.
type Status struct {
	Profile       string            `json:"profile"`
	Topics        string            `json:"topics"`          // the topic LLM
	LLMURL        string            `json:"llm_url"`         // where it runs
	TopicAPI      string            `json:"topic_api"`       // the external topic model, "" when not set up
	NoLocalTopics bool              `json:"no_local_topics"` // TOPIC_MODEL=external: Ollama is left out
	Speech        string            `json:"speech"`          // the speech-to-text server
	ByLang        map[string]string `json:"by_lang"`         // per-language speech servers
	Segment       int               `json:"segment"`         // seconds of audio per transcription
	Images        string            `json:"images"`          // the image provider in use, filled in per request
	Locked        bool              `json:"locked"`          // PANEL_TOKEN is set
	Saved         bool              `json:"saved"`           // changes are saved to a settings file
	Started       time.Time         `json:"started"`
	Password      bool              `json:"password"` // filled in per request
	Env           []EnvRow          `json:"env"`      // what .env sets, shown read-only
}

// EnvRow is one thing .env sets, as the page shows it: never a key itself,
// only whether it is set.
type EnvRow struct {
	Label string `json:"label"`
	Var   string `json:"var"`
	Value string `json:"value"`
}

// Mic is the microphone as the page shows it: names and a loudness, never
// audio.
type Mic struct {
	Live    bool           `json:"live"`            // listening on a microphone (not a -wav or -script run)
	Devices []audio.Device `json:"devices"`         // the inputs the machine offers
	Active  string         `json:"active"`          // the one in use, "" when it has no name or none is open
	Level   *float64       `json:"level,omitempty"` // its loudness just now in dBFS; nil when paused or not open
	Error   string         `json:"error,omitempty"` // why no microphone is open, or why the list is empty
}

// Server serves the page under the same /<token>/ prefix as the panel.
type Server struct {
	Token    string
	Auth     *Auth // the optional password; nil means none
	Pipeline *pipeline.Pipeline
	Panel    *panel.Server
	Status   Status
	// Images are the image providers, ready or not; image_provider picks one.
	Images []imagegen.Choice
	// Apply checks and applies new settings and returns them with defaults
	// filled in, or says why they are wrong.
	Apply func(config.Settings) (config.Settings, error)
	// Draft draws the page as it would look with unsaved settings, or says
	// why they are wrong; may be nil.
	Draft func(config.Settings) (image.Image, error)
	// Logs is E-maki's own recent log, for System > Logs; may be nil.
	Logs *LogRing
	// Mic says what E-maki listens on right now; may be nil.
	Mic func() Mic
	// Save keeps applied settings for the next start; may be nil.
	Save func(config.Settings) error
	// Health says whether speech-to-text, the topic model and the image
	// provider answer.
	Health *health.Tracker
	// TestPrompt is what a test picture is made from; empty uses
	// DefaultTestPrompt.
	TestPrompt string

	// TestPicturesPerDay is how many test pictures a provider that is not
	// on this machine may make in a day; 0 is no limit.
	TestPicturesPerDay int

	testing  sync.Mutex // one test picture at a time
	testDay  string     // the day testMade counts, guarded by testing
	testMade int
	// Timeline holds the saved pictures for Pictures > History; nil without
	// a settings file.
	Timeline *timeline.Store
	// TimelineRow is the height of a row of pictures on Pictures > History.
	TimelineRow int

	applying   sync.Mutex // one change at a time, applied and saved in order
	cookiePath string
	mu         sync.Mutex
	settings   config.Settings
}

// SetSettings records the settings in force at start.
func (s *Server) SetSettings(c config.Settings) {
	s.mu.Lock()
	s.settings = c
	s.mu.Unlock()
}

func (s *Server) Register(mux *http.ServeMux) {
	pre := ""
	if s.Token != "" {
		pre = "/" + s.Token
		mux.Handle("GET "+pre+"", http.RedirectHandler(pre+"/", http.StatusFound))
	}
	files, _ := fs.Sub(static, "static")
	page := func(name string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			b, _ := fs.ReadFile(files, name)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(b)
		}
	}
	mux.HandleFunc("GET "+pre+"/{$}", page("index.html"))
	mux.HandleFunc("GET "+pre+"/view", page("view.html"))
	assets := http.StripPrefix(pre+"/ui/", http.FileServerFS(files))
	mux.Handle("GET "+pre+"/ui/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		assets.ServeHTTP(w, r)
	}))
	// The picture is as open as /frame.png, so a kiosk screen needs no
	// password; everything else on the page does when one is set.
	mux.HandleFunc("GET "+pre+"/preview.png", s.preview)
	mux.HandleFunc("GET "+pre+"/api/page", func(w http.ResponseWriter, r *http.Request) {
		s.reply(w, map[string]string{"page": s.Panel.ETag()})
	})
	s.cookiePath = pre + "/"
	mux.HandleFunc("POST "+pre+"/api/login", s.guard(s.login))
	mux.HandleFunc("POST "+pre+"/api/logout", s.guard(func(w http.ResponseWriter, r *http.Request) {
		SignOut(w, s.cookiePath)
		w.WriteHeader(http.StatusNoContent)
	}))
	mux.HandleFunc("GET "+pre+"/api/state", s.private(s.state))
	mux.HandleFunc("GET "+pre+"/api/mic", s.private(func(w http.ResponseWriter, r *http.Request) {
		m := Mic{Devices: []audio.Device{}}
		if s.Mic != nil {
			m = s.Mic()
		}
		s.reply(w, m)
	}))
	mux.HandleFunc("GET "+pre+"/api/frame", s.private(func(w http.ResponseWriter, r *http.Request) {
		s.reply(w, s.Panel.Seen())
	}))
	mux.HandleFunc("PUT "+pre+"/api/settings", s.guard(s.private(s.putSettings)))
	mux.HandleFunc("GET "+pre+"/api/preview.png", s.private(s.draft))
	mux.HandleFunc("POST "+pre+"/api/tiles/{action}", s.guard(s.private(s.tile)))
	mux.HandleFunc("POST "+pre+"/api/queue/{action}", s.guard(s.private(s.line)))
	mux.HandleFunc("POST "+pre+"/api/redraw", s.guard(s.private(s.redraw)))
	mux.HandleFunc("POST "+pre+"/api/test-picture", s.guard(s.private(s.testPicture)))
	mux.HandleFunc("POST "+pre+"/api/removed/allow", s.guard(s.private(s.allow)))
	mux.HandleFunc("GET "+pre+"/api/picture.png", s.private(s.picture))
	mux.HandleFunc("GET "+pre+"/api/logs", s.private(func(w http.ResponseWriter, r *http.Request) {
		after, _ := strconv.Atoi(r.URL.Query().Get("after"))
		lines := []LogLine{}
		if s.Logs != nil {
			lines = s.Logs.Since(after)
		}
		s.reply(w, lines)
	}))
	// The same lines as a file, for a bug report.
	mux.HandleFunc("GET "+pre+"/api/logs.txt", s.private(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="emaki.log"`)
		if s.Logs == nil {
			return
		}
		for _, l := range s.Logs.Since(0) {
			fmt.Fprintf(w, "%s %s\n", l.At.Format(time.DateTime), l.Text)
		}
	}))
	mux.HandleFunc("POST "+pre+"/api/pause", s.guard(s.private(s.pause)))
	mux.HandleFunc("POST "+pre+"/api/clear", s.guard(s.private(s.clear)))
	mux.HandleFunc("POST "+pre+"/api/password", s.guard(s.private(s.password)))
	mux.HandleFunc("GET "+pre+"/api/timeline", s.private(s.timeline))
	mux.HandleFunc("GET "+pre+"/api/timeline/{id}", s.private(s.timelinePicture))
	mux.HandleFunc("POST "+pre+"/api/timeline/keep", s.guard(s.private(s.timelineKeep)))
	mux.HandleFunc("POST "+pre+"/api/timeline/delete", s.guard(s.private(s.timelineDelete)))
}

// private lets a request through only when signed in (or no password is
// set); otherwise it answers 401 so the page shows the sign-in form.
func (s *Server) private(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.Auth.SignedIn(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"signin":true}` + "\n"))
			return
		}
		next(w, r)
	}
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if !s.Auth.Enabled() {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	ok, wait := s.Auth.Check(r, body.Password)
	if wait > 0 {
		http.Error(w, fmt.Sprintf("Too many wrong passwords. Try again in %d seconds.", int(wait.Seconds())+1), http.StatusTooManyRequests)
		return
	}
	if !ok {
		log.Print("settings page: wrong password")
		http.Error(w, "That password is not right.", http.StatusUnauthorized)
		return
	}
	s.Auth.SignIn(w, s.cookiePath)
	w.WriteHeader(http.StatusNoContent)
}

// password sets, changes or removes the password. Changing or removing one
// needs the current password too, so an unlocked browser left open cannot
// lock its owner out.
func (s *Server) password(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Current  string `json:"current"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `send {"current": "...", "password": "..."}`, http.StatusBadRequest)
		return
	}
	if s.Auth == nil {
		http.Error(w, "passwords are not available", http.StatusNotImplemented)
		return
	}
	if s.Auth.Enabled() {
		if ok, wait := s.Auth.Check(r, body.Current); !ok {
			msg := "The current password is not right."
			if wait > 0 {
				msg = fmt.Sprintf("Too many wrong passwords. Try again in %d seconds.", int(wait.Seconds())+1)
			}
			http.Error(w, msg, http.StatusForbidden)
			return
		}
	}
	if body.Password != "" && len(body.Password) < 8 {
		http.Error(w, "Use at least 8 characters.", http.StatusUnprocessableEntity)
		return
	}
	if err := s.Auth.Set(body.Password); err != nil {
		http.Error(w, "Could not save the password: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if body.Password != "" {
		log.Print("settings page: password set")
		s.Auth.SignIn(w, s.cookiePath)
	} else {
		log.Print("settings page: password removed")
		SignOut(w, s.cookiePath)
	}
	s.reply(w, s.snapshot())
}

func init() {
	// Some minimal Linux images have no mime table: module scripts must be
	// served as JavaScript or the browser refuses them.
	mime.AddExtensionType(".mjs", "text/javascript")
}

// Protect wraps everything E-maki serves. It sets the headers below on
// every answer, and refuses a request whose Host is a public domain name: a
// web page on the internet can point its own domain at this machine (DNS
// rebinding), and the browser would then let it read and change the settings
// page. An IP address, localhost, a name without a dot and names ending in
// .local, .lan, .internal or .home.arpa are let through, and so is any name
// in hosts (EMAKI_HOSTS), for a reverse proxy or a VPN name.
func Protect(next http.Handler, hosts []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers(w)
		if !localHost(r.Host, hosts) {
			http.Error(w, "E-maki does not answer to this address. Open it by IP address, or add the name to EMAKI_HOSTS in .env.", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func localHost(hostport string, extra []string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.ToLower(strings.Trim(host, "[]"))
	if net.ParseIP(host) != nil || host == "localhost" || slices.Contains(extra, host) {
		return true
	}
	for _, suffix := range []string{".local", ".lan", ".internal", ".home.arpa"} {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return host != "" && !strings.Contains(host, ".")
}

// headers keep the page to itself: no framing by other sites, no scripts
// from elsewhere, and no Referer, which would leak the token in the URL.
func headers(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Content-Type-Options", "nosniff")
}

// guard lets a change through only from the page itself: JSON (which a
// form on another site cannot send without the browser asking first) from
// the same origin. Without it any web page open on the LAN could change the
// frame's settings.
func (s *Server) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" {
			http.Error(w, "send JSON", http.StatusUnsupportedMediaType)
			return
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
			http.Error(w, "changes come from the settings page only", http.StatusForbidden)
			return
		}
		if o := r.Header.Get("Origin"); o != "" {
			if u, err := url.Parse(o); err != nil || u.Host != r.Host {
				http.Error(w, "changes come from the settings page only", http.StatusForbidden)
				return
			}
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
		next(w, r)
	}
}

func (s *Server) preview(w http.ResponseWriter, r *http.Request) {
	etag := s.Panel.ETag()
	w.Header().Set("Cache-Control", "no-cache")
	if etag != "" && r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("ETag", etag)
	w.Header().Set("Content-Type", "image/png")
	if ok, err := s.Panel.Preview(w); !ok {
		http.Error(w, "no page yet", http.StatusNotFound)
	} else if err != nil {
		log.Printf("preview: %v", err)
	}
}

// draft answers with the frame's picture as unsaved layout settings would
// make it: the settings in force with the query's layout, tiles,
// hide_captions, arrange, spacing, background, shape, rotate_minutes and
// half_life_hours over them. It changes nothing.
func (s *Server) draft(w http.ResponseWriter, r *http.Request) {
	if s.Draft == nil {
		http.Error(w, "no preview of unsaved settings", http.StatusNotImplemented)
		return
	}
	s.mu.Lock()
	set := s.settings
	s.mu.Unlock()
	q := r.URL.Query()
	var err error
	if q.Has("layout") {
		set.Layout = q.Get("layout")
	}
	if v := q.Get("tiles"); v != "" && err == nil {
		set.Tiles, err = strconv.Atoi(v)
	}
	if v := q.Get("half_life_hours"); v != "" && err == nil {
		set.HalfLifeHours, err = strconv.ParseFloat(v, 64)
	}
	if v := q.Get("hide_captions"); v != "" && err == nil {
		set.HideCaptions, err = strconv.ParseBool(v)
	}
	for k, dst := range map[string]*string{"arrange": &set.Arrange, "spacing": &set.Spacing, "background": &set.Background, "shape": &set.Shape} {
		if q.Has(k) {
			*dst = q.Get(k)
		}
	}
	for k, dst := range map[string]*int{"rotate_minutes": &set.RotateMinutes, "panel_width": &set.PanelWidth, "panel_height": &set.PanelHeight, "panel_rotation": &set.PanelRotation} {
		if v := q.Get(k); v != "" && err == nil {
			*dst, err = strconv.Atoi(v)
		}
	}
	var page image.Image
	if err == nil {
		page, err = s.Draft(set)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	if err := png.Encode(w, panel.Dither(page)); err != nil {
		log.Printf("preview: %v", err)
	}
}

type state struct {
	Settings      config.Settings           `json:"settings"`
	Styles        []imagegen.Style          `json:"styles"`
	DefaultStyle  string                    `json:"default_style"`
	Categories    []string                  `json:"categories"`
	Languages     [][2]string               `json:"languages"` // what Whisper knows: code, name
	Wall          []pipeline.Tile           `json:"wall"`
	Queue         []pipeline.Tile           `json:"queue"`     // pictures waiting to go up, the next one first
	Removed       []memory.Removal          `json:"removed"`   // removed topics still kept away
	MaxTiles      int                       `json:"max_tiles"` // tiles the current layout shows
	Waiting       int                       `json:"waiting"`   // new topics waiting to go up
	NextAt        *time.Time                `json:"next_at,omitempty"`
	ImagesToday   int                       `json:"images_today"`
	Collected     int                       `json:"collected"`            // diary topics waiting for the next painting
	DiaryNext     *time.Time                `json:"diary_next,omitempty"` // when diary mode paints next
	Page          string                    `json:"page"`                 // the page's ETag, "" before the first one
	Activity      pipeline.Activity         `json:"activity"`
	DefaultPrompt string                    `json:"default_prompt"`
	Providers     []imagegen.Choice         `json:"providers"`
	Status        Status                    `json:"status"`
	Health        map[string]health.Service `json:"health"`
	Warning       string                    `json:"warning,omitempty"`
}

func (s *Server) snapshot() state {
	s.mu.Lock()
	set := s.settings
	s.mu.Unlock()
	if set.BlockSensitive == nil {
		set.BlockSensitive = []string{}
	}
	if set.Languages == nil {
		set.Languages = []string{}
	}
	tiles := set.Tiles
	if set.Layout == "single" {
		tiles = 1
	}
	st := state{
		Settings: set, Styles: imagegen.Styles, DefaultStyle: imagegen.DefaultStyle,
		Categories: config.Categories, Languages: stt.Languages, Wall: s.Pipeline.Wall(), Queue: s.Pipeline.Waiting(), Removed: s.Pipeline.Removed(), MaxTiles: min(tiles, collage.MaxTiles),
		ImagesToday: s.Pipeline.ImagesToday(), Page: s.Panel.ETag(), Status: s.Status,
		Activity: s.Pipeline.Activity(), DefaultPrompt: topic.DefaultPrompt, Providers: s.Images,
	}
	st.Status.Password = s.Auth.Enabled()
	st.Health = map[string]health.Service{}
	for _, n := range []string{health.Speech, health.Topics, health.Images} {
		st.Health[n] = s.Health.Get(n)
	}
	if len(s.Images) > 0 {
		in, _ := imagegen.Pick(s.Images, set.ImageProvider)
		st.Status.Images = in.Name
	}
	if n, next := s.Pipeline.NextEdition(); !next.IsZero() {
		st.Collected, st.DiaryNext = n, &next
	}
	if n, next := s.Pipeline.Queue(); n > 0 {
		st.Waiting, st.NextAt = n, &next
	}
	return st
}

func (s *Server) state(w http.ResponseWriter, r *http.Request) { s.reply(w, s.snapshot()) }

func (s *Server) reply(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(v)
}

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	var set config.Settings
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&set); err != nil {
		http.Error(w, "settings: "+err.Error(), http.StatusBadRequest)
		return
	}
	s.change(w, func(config.Settings) config.Settings { return set })
}

// change applies and saves an edit of the current settings, one at a time.
func (s *Server) change(w http.ResponseWriter, edit func(config.Settings) config.Settings) {
	st, err := s.edit(edit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	s.reply(w, st)
}

func (s *Server) edit(edit func(config.Settings) config.Settings) (state, error) {
	s.applying.Lock()
	defer s.applying.Unlock()
	s.mu.Lock()
	cur := s.settings
	s.mu.Unlock()
	set, err := s.Apply(edit(cur))
	if err != nil {
		return state{}, err
	}
	s.SetSettings(set)
	log.Print("settings changed")
	st := s.snapshot()
	if s.Save != nil {
		if err := s.Save(set); err != nil {
			log.Printf("settings: %v", err)
			st.Warning = "Applied, but not saved: " + err.Error() + ". They last until E-maki restarts."
		}
	}
	return st, nil
}

// Press handles a button on the frame, as panel_buttons sets it: "pause"
// pauses or resumes listening, like the switch on the page, and the rest go
// to the pipeline.
func (s *Server) Press(button string) {
	s.mu.Lock()
	action := s.settings.Button(button)
	s.mu.Unlock()
	if action != "pause" {
		s.Pipeline.Press(action)
		return
	}
	st, err := s.edit(func(c config.Settings) config.Settings { c.Paused = !c.Paused; return c })
	if err != nil {
		log.Printf("button %s: %v", button, err)
		return
	}
	log.Printf("button %s: paused %v", button, st.Settings.Paused)
}

// pause stops or starts listening at once, whatever else is being edited.
func (s *Server) pause(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Paused bool `json:"paused"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `send {"paused": true}`, http.StatusBadRequest)
		return
	}
	s.change(w, func(c config.Settings) config.Settings { c.Paused = body.Paused; return c })
}

func (s *Server) clear(w http.ResponseWriter, r *http.Request) {
	s.Pipeline.Clear()
	s.reply(w, s.snapshot())
}

func (s *Server) tile(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Label string `json:"label"`
		To    string `json:"to"` // the new label, for rename
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Label == "" {
		http.Error(w, "send {\"label\": \"...\"}", http.StatusBadRequest)
		return
	}
	var found bool
	switch r.PathValue("action") {
	case "pin":
		found = s.Pipeline.Pin(body.Label, true)
	case "unpin":
		found = s.Pipeline.Pin(body.Label, false)
	case "remove":
		found = s.Pipeline.Remove(body.Label)
	case "rename":
		to := strings.Join(strings.Fields(body.To), " ")
		if n := len(strings.Fields(to)); n == 0 || n > topic.MaxTopicWords {
			http.Error(w, fmt.Sprintf("a caption has 1 to %d words", topic.MaxTopicWords), http.StatusBadRequest)
			return
		}
		if !s.Pipeline.Rename(body.Label, to) {
			http.Error(w, "that topic is gone, or another one already has that caption", http.StatusConflict)
			return
		}
		found = true
	default:
		http.Error(w, "action is pin, unpin, remove or rename", http.StatusNotFound)
		return
	}
	if !found {
		http.Error(w, "that topic is no longer on the frame", http.StatusNotFound)
		return
	}
	s.reply(w, s.snapshot())
}

// allow lets a removed topic come back.
func (s *Server) allow(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Label string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Label == "" {
		http.Error(w, "send {\"label\": \"...\"}", http.StatusBadRequest)
		return
	}
	if !s.Pipeline.Allow(body.Label) {
		http.Error(w, "that topic is not kept away", http.StatusNotFound)
		return
	}
	s.reply(w, s.snapshot())
}

// line moves, puts up or drops a picture waiting in the queue.
func (s *Server) line(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Label string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Label == "" {
		http.Error(w, "send {\"label\": \"...\"}", http.StatusBadRequest)
		return
	}
	a := r.PathValue("action")
	if a != "up" && a != "now" && a != "drop" {
		http.Error(w, "action is up, now or drop", http.StatusNotFound)
		return
	}
	if !s.Pipeline.Line(a, body.Label) {
		http.Error(w, "that picture is no longer waiting", http.StatusNotFound)
		return
	}
	s.reply(w, s.snapshot())
}

// picture answers with one topic's picture, small and dithered as the
// frame would show it.
func (s *Server) picture(w http.ResponseWriter, r *http.Request) {
	img := s.Pipeline.Picture(r.URL.Query().Get("label"))
	if img == nil {
		http.Error(w, "no such picture", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	png.Encode(w, panel.Dither(small(img, 320)))
}

// slow lets a handler that waits for the image provider answer later than
// the server's write timeout: a picture drawn on a CPU can take minutes.
func slow(w http.ResponseWriter) {
	http.NewResponseController(w).SetWriteDeadline(time.Now().Add(25 * time.Minute))
}

// redraw makes a new picture for a tile or a waiting picture.
func (s *Server) redraw(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Label string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Label == "" {
		http.Error(w, "send {\"label\": \"...\"}", http.StatusBadRequest)
		return
	}
	slow(w)
	if err := s.Pipeline.Redraw(r.Context(), body.Label); err != nil {
		code := http.StatusBadGateway
		switch {
		case errors.Is(err, pipeline.ErrGone):
			code = http.StatusNotFound
		case errors.Is(err, pipeline.ErrBusy):
			code = http.StatusConflict
		}
		log.Printf("redraw: %v", err)
		http.Error(w, "Not redrawn: "+err.Error()+".", code)
		return
	}
	s.reply(w, s.snapshot())
}

// DefaultTestPrompt is the description a test picture is made from when
// the profile sets none.
const DefaultTestPrompt = "A cheerful cartoon teapot pouring tea into a cup on a kitchen table, steam rising."

// testPicture makes one picture with the provider and style the page has
// picked, saved or not, and answers with it as the frame would show it. It
// does not count against the daily cap (it has its own, TestPicturesPerDay)
// and goes nowhere near the frame.
func (s *Server) testPicture(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Provider string `json:"provider"`
		Style    string `json:"style"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `send {"provider": "...", "style": "..."}`, http.StatusBadRequest)
		return
	}
	if err := imagegen.Check(s.Images, body.Provider); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	if !s.testing.TryLock() {
		http.Error(w, "A test picture is already being made.", http.StatusConflict)
		return
	}
	defer s.testing.Unlock()
	c, _ := imagegen.Pick(s.Images, body.Provider)
	// Test pictures cost money like any other, so they have a daily limit
	// of their own. A provider on this machine costs nothing.
	if !c.Local && s.TestPicturesPerDay > 0 {
		if day := time.Now().Format(time.DateOnly); day != s.testDay {
			s.testDay, s.testMade = day, 0
		}
		if s.testMade >= s.TestPicturesPerDay {
			http.Error(w, fmt.Sprintf("Today's limit of %d test pictures is used up. It starts again tomorrow; test_pictures_per_day in the config profile changes it.", s.TestPicturesPerDay), http.StatusTooManyRequests)
			return
		}
		s.testMade++
	}
	slow(w)
	// Only a test of the provider in use counts for its status.
	var tracker *health.Tracker
	s.mu.Lock()
	if cur, _ := imagegen.Pick(s.Images, s.settings.ImageProvider); cur.ID == c.ID {
		tracker = s.Health
	}
	s.mu.Unlock()
	log.Printf("test picture with %s", c.Name)
	start := time.Now()
	prompt := cmp.Or(s.TestPrompt, DefaultTestPrompt)
	img, err := health.Provider{P: c.New(body.Style), Tracker: tracker}.Generate(r.Context(), prompt)
	if err != nil {
		log.Printf("test picture: %v", err)
		http.Error(w, c.Name+" did not make the picture: "+err.Error(), http.StatusBadGateway)
		return
	}
	took := time.Since(start).Seconds()
	log.Printf("test picture: done in %.1fs", took)
	var buf bytes.Buffer
	if err := png.Encode(&buf, panel.Dither(small(img, 480))); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.reply(w, map[string]any{"image": "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()),
		"took": took, "provider": c.Name, "prompt": prompt})
}

// small scales a picture down to width w.
func small(img image.Image, w int) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, w, w*b.Dy()/max(1, b.Dx())))
	xdraw.ApproxBiLinear.Scale(out, out.Bounds(), img, b, xdraw.Src, nil)
	return out
}

// timelineState is what Pictures > History shows: whether new pictures are
// saved, and the saved ones, newest first.
type timelineState struct {
	Saved    bool             `json:"saved"` // there is a folder to save to
	On       bool             `json:"on"`
	RowPx    int              `json:"row_px"`
	Pictures []timeline.Entry `json:"pictures"`
}

func (s *Server) timelineSnapshot() timelineState {
	s.mu.Lock()
	on := s.settings.TimelineOn()
	s.mu.Unlock()
	st := timelineState{Saved: s.Timeline != nil, On: on && s.Timeline != nil, RowPx: s.TimelineRow, Pictures: []timeline.Entry{}}
	if s.Timeline != nil {
		st.Pictures = s.Timeline.List()
	}
	return st
}

func (s *Server) timeline(w http.ResponseWriter, r *http.Request) { s.reply(w, s.timelineSnapshot()) }

// timelinePicture answers with a saved picture, or its thumbnail with
// ?size=thumb. A saved picture never changes, so browsers may keep it.
func (s *Server) timelinePicture(w http.ResponseWriter, r *http.Request) {
	path := ""
	if s.Timeline != nil {
		path = s.Timeline.File(r.PathValue("id"), r.URL.Query().Get("size") == "thumb")
	}
	if path == "" {
		http.Error(w, "no such picture", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	http.ServeFile(w, r, path)
}

// timelineKeep switches saving new pictures on or off. Pictures already
// saved stay until they are deleted.
func (s *Server) timelineKeep(w http.ResponseWriter, r *http.Request) {
	var body struct {
		On bool `json:"on"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `send {"on": true}`, http.StatusBadRequest)
		return
	}
	if s.Timeline == nil {
		http.Error(w, "the timeline needs a settings file: start E-maki with -settings", http.StatusConflict)
		return
	}
	s.change(w, func(c config.Settings) config.Settings { c.Timeline = &body.On; return c })
}

func (s *Server) timelineDelete(w http.ResponseWriter, r *http.Request) {
	if s.Timeline != nil {
		if err := s.Timeline.DeleteAll(); err != nil {
			http.Error(w, "Could not delete the pictures: "+err.Error(), http.StatusInternalServerError)
			return
		}
		log.Print("timeline: all pictures deleted")
	}
	s.reply(w, s.timelineSnapshot())
}
