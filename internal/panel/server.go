package panel

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"image"
	"image/png"
	"io"
	"net/http"
	"sync"
)

// Server serves the current page with Fugleramme's panel protocol, so its
// Inky (examples/inky/main.py) and EE02 clients can show E-kiru unchanged:
//
//	GET  /frame.png  the dithered 6-colour page, with ETag / 304
//	GET  /frame.e6   the same page packed for the 13.3" glass
//	POST /press      button=A|B|C|D from the panel's buttons
//
// With a Token the three paths live under /<token>/, so a frame is set up
// with FRAME_URL=http://brain:8090/<token> and nothing else on the network
// can read the page or press its buttons.
type Server struct {
	Token    string
	Rotation int
	Poll     int                 // minutes the EE02 client sleeps between fetches
	OnPress  func(button string) // may be nil

	mu   sync.RWMutex
	page *image.Paletted // dithered, upright
	png  []byte
	e6   []byte
	etag string
}

// SetPage dithers a full-colour page and makes it the current frame.
func (s *Server) SetPage(page image.Image) error {
	p := Dither(page)
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.encode(p)
}

// SetFrame changes the rotation and poll interval, and re-encodes the
// current page for them.
func (s *Server) SetFrame(rotation, poll int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Rotation, s.Poll = rotation, poll
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
		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		b := r.FormValue("button")
		if b != "A" && b != "B" && b != "C" && b != "D" {
			http.Error(w, "button must be A, B, C or D", http.StatusBadRequest)
			return
		}
		if s.OnPress != nil {
			s.OnPress(b)
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request, ctype string, e6 bool) {
	s.mu.RLock()
	body, etag, drawn := s.png, s.etag, s.png != nil
	if e6 {
		body = s.e6
	}
	s.mu.RUnlock()
	if body == nil && drawn {
		http.Error(w, `/frame.e6 is only for the 7.3" and 13.3" panels`, http.StatusNotFound)
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
