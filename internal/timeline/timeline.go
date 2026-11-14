// Package timeline keeps every picture E-kiru made, with its topic label,
// picture description, style and time, so the settings page can show them
// day by day. It never holds what was said.
package timeline

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	xdraw "golang.org/x/image/draw"
)

// Entry is one picture in the timeline.
type Entry struct {
	ID     string    `json:"id"`
	Label  string    `json:"label"`
	Prompt string    `json:"prompt"` // the description sent to the image API
	Style  string    `json:"style"`
	Made   time.Time `json:"made"`
	W      int       `json:"w"`
	H      int       `json:"h"`
}

const index = "timeline.json"

// Options say how pictures are saved.
type Options struct {
	ThumbPx int // longest side of a thumbnail
	Quality int // JPEG quality, 1 to 100
}

var validID = regexp.MustCompile(`^[0-9]{8}-[0-9]{6}-[0-9a-f]{8}$`)

// Store is the timeline folder. It is safe for concurrent use.
type Store struct {
	dir     string
	opt     Options
	mu      sync.Mutex
	entries []Entry // oldest first
}

// Open reads the timeline in dir. A missing folder is an empty timeline.
func Open(dir string, opt Options) (*Store, error) {
	s := &Store{dir: dir, opt: opt}
	b, err := os.ReadFile(filepath.Join(dir, index))
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(b, &s.entries); err != nil {
		return s, fmt.Errorf("%s: %w", index, err)
	}
	return s, nil
}

// Add saves a picture as a JPEG with a thumbnail and lists it.
func (s *Store) Add(label, prompt, style string, made time.Time, img image.Image) (Entry, error) {
	h := fnv.New32a()
	fmt.Fprintf(h, "%s|%d", label, made.UnixNano())
	b := img.Bounds()
	e := Entry{ID: fmt.Sprintf("%s-%08x", made.UTC().Format("20060102-150405"), h.Sum32()),
		Label: label, Prompt: prompt, Style: style, Made: made, W: b.Dx(), H: b.Dy()}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return e, err
	}
	if err := s.writeJPEG(s.file(e.ID, false), img); err != nil {
		return e, err
	}
	scale := float64(s.opt.ThumbPx) / float64(max(b.Dx(), b.Dy(), 1))
	small := image.NewRGBA(image.Rect(0, 0, max(1, int(float64(b.Dx())*scale)), max(1, int(float64(b.Dy())*scale))))
	xdraw.ApproxBiLinear.Scale(small, small.Bounds(), img, b, xdraw.Src, nil)
	if err := s.writeJPEG(s.file(e.ID, true), small); err != nil {
		return e, err
	}
	s.entries = append(s.entries, e)
	return e, s.saveIndex()
}

// List returns the pictures, newest first.
func (s *Store) List() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := slices.Clone(s.entries)
	slices.Reverse(out)
	if out == nil {
		out = []Entry{}
	}
	return out
}

// File is the path of a listed picture, or its thumbnail; "" when there is
// no such picture.
func (s *Store) File(id string, thumb bool) string {
	if !validID.MatchString(id) {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !slices.ContainsFunc(s.entries, func(e Entry) bool { return e.ID == id }) {
		return ""
	}
	return s.file(id, thumb)
}

// DeleteAll removes every picture and the index.
func (s *Store) DeleteAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = nil
	files, err := os.ReadDir(s.dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, f := range files {
		if n := f.Name(); n == index || strings.HasSuffix(n, ".jpg") || strings.HasSuffix(n, ".tmp") {
			if err := os.Remove(filepath.Join(s.dir, n)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) file(id string, thumb bool) string {
	if thumb {
		return filepath.Join(s.dir, id+".thumb.jpg")
	}
	return filepath.Join(s.dir, id+".jpg")
}

func (s *Store) saveIndex() error {
	b, err := json.MarshalIndent(s.entries, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(s.dir, index), b)
}

func (s *Store) writeJPEG(path string, img image.Image) error {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: s.opt.Quality}); err != nil {
		return err
	}
	return writeFile(path, buf.Bytes())
}

// writeFile writes through a temporary file, so a crash never leaves half a file.
func writeFile(path string, b []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
