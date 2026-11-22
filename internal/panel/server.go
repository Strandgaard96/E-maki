package panel

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Server serves the current page with Fugleramme's panel protocol, so its
// Inky (examples/inky/main.py) and EE02 clients can show E-maki unchanged:
//
//	GET  /frame.png  the dithered 6-colour page, with ETag / 304
//	GET  /frame.e6   the same page packed for the 13.3" glass
//	POST /press      button=A to H from the panel's buttons (Inky: A to D,
//	                 Fugleramme's XIAO Arduino firmware: A to C)
//
// With a Token the three paths live under /<token>/, so a frame is set up
// with FRAME_URL=http://server:8090/<token> and nothing else on the network
// can read the page or press its buttons.
type Server struct {
	Token    string
	Rotation int
	Poll     int                 // minutes the EE02 client sleeps between fetches
	Inks     color.Palette       // the panel's inks; nil is Palette
	OnPress  func(button string) // may be nil

	mu   sync.RWMutex
	seen Seen
	page *image.Paletted // dithered, upright
	png  []byte
	e6   []byte
	etag string
}

// Seen is what the frame last did, kept in memory only, so the settings
// page can show which frame is on the wall and that its buttons get through.
type Seen struct {
	Client  string     `json:"client"`            // see client; "" before any fetch
	Fetched *time.Time `json:"fetched,omitempty"` // its last fetch of the page
	Button  string     `json:"button,omitempty"`  // its last press
	Pressed *time.Time `json:"pressed,omitempty"`
}

// client names the frame software behind a request by its User-Agent:
// "xiao" is Fugleramme's XIAO Arduino firmware (sends A to C), "inky" its
// Inky client on a Pi (A to D), "esp32" its MicroPython EE02 and EE04
// clients, which send no User-Agent and no presses, and "other" anything
// else. A browser is not a frame and gives "".
func client(r *http.Request) string {
	ua := r.UserAgent()
	switch {
	case strings.HasPrefix(ua, "ESP32HTTPClient"):
		return "xiao"
	case strings.HasPrefix(ua, "Python-urllib"):
		return "inky"
	case ua == "":
		return "esp32"
	case strings.HasPrefix(ua, "Mozilla/"):
		return ""
	}
	return "other"
}

// Seen returns what the frame last did.
func (s *Server) Seen() Seen {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.seen
}

// saw notes a fetch or, with a button, a press.
func (s *Server) saw(r *http.Request, button string) {
	c := client(r)
	if c == "" {
		return
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seen.Client = c
	if button == "" {
		s.seen.Fetched = &now
	} else {
		s.seen.Button, s.seen.Pressed = button, &now
	}
}

// SetPage dithers a full-colour page and makes it the current frame.
func (s *Server) SetPage(page image.Image) error {
	s.mu.RLock()
	inks := s.Inks
	s.mu.RUnlock()
	p := Dither(page, inks)
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.encode(p)
}

// SetFrame changes the rotation, poll interval and inks, and re-encodes the
// current page for them. New inks apply from the next SetPage.
func (s *Server) SetFrame(rotation, poll int, inks color.Palette) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Rotation, s.Poll, s.Inks = rotation, poll, inks
	if s.page == nil {
		return nil
	}
	return s.encode(s.page)
}

// encode needs s.mu held.
func (s *Server) encode(p *image.Paletted) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, rotateCCW(p, s.Rotation)); err != nil {
		return err
	}
	sum := md5.Sum(buf.Bytes())
	s.page, s.png, s.e6, s.etag = p, buf.Bytes(), EncodeE6(p, s.Rotation, s.Poll), `"`+hex.EncodeToString(sum[:])+`"`
	return nil
}

// Preview writes the current page upright, as people see it on the wall,
// whatever way the panel is turned. ok is false before the first page.
func (s *Server) Preview(w io.Writer) (ok bool, err error) {
	s.mu.RLock()
	p := s.page
	s.mu.RUnlock()
	if p == nil {
		return false, nil
	}
	return true, png.Encode(w, p)
}

// ETag of the current page, "" before the first one.
func (s *Server) ETag() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.etag
}

func (s *Server) Register(mux *http.ServeMux) {
	pre := ""
	if s.Token != "" {
		pre = "/" + s.Token
	}
	mux.HandleFunc("GET "+pre+"/frame.png", func(w http.ResponseWriter, r *http.Request) { s.serve(w, r, "image/png", false) })
	mux.HandleFunc("GET "+pre+"/frame.e6", func(w http.ResponseWriter, r *http.Request) {
		s.serve(w, r, "application/octet-stream", true)
	})
	mux.HandleFunc("POST "+pre+"/press", func(w http.ResponseWriter, r *http.Request) {
		// A frame sends neither header; a web page on another site sends
		// both, and must not press the buttons.
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
			http.Error(w, "buttons are pressed on the frame only", http.StatusForbidden)
			return
		}
		if o := r.Header.Get("Origin"); o != "" {
			if u, err := url.Parse(o); err != nil || u.Host != r.Host {
				http.Error(w, "buttons are pressed on the frame only", http.StatusForbidden)
				return
			}
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		b := r.FormValue("button")
		if len(b) != 1 || b < "A" || b > "H" {
			http.Error(w, "button must be a letter from A to H", http.StatusBadRequest)
			return
		}
		s.saw(r, b)
		if s.OnPress != nil {
			s.OnPress(b)
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request, ctype string, e6 bool) {
	s.saw(r, "")
	s.mu.RLock()
	body, etag, drawn := s.png, s.etag, s.png != nil
	if e6 {
		body = s.e6
	}
	s.mu.RUnlock()
	if body == nil && drawn {
		http.Error(w, `/frame.e6 is only for the 7.3" and 13.3" Spectra 6 panels`, http.StatusNotFound)
		return
	}
	if body == nil {
		http.Error(w, "no page yet", http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", ctype)
	w.Write(body)
}
