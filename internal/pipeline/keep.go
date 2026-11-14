package pipeline

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/Strandgaard96/e-kiru/internal/memory"
)

// KeepDir, when set, is where the pictures on the frame and in the queue
// are saved with their topics, picture descriptions and styles, so the
// frame comes back after a restart. In diary mode the topics collected for
// the next painting are saved too, as labels and picture descriptions
// without a picture. Never what was said. Empty keeps
// everything in memory only.

type kept struct {
	memory.Kept
	Style string `json:"style"`
	File  string `json:"file"`
}

const (
	keepIndex  = "pictures.json"
	keepCounts = "counts.json" // pictures made per day and removed topics (memory.Counts)
)

// keep saves the pictures to KeepDir. It needs p.mu held. A picture file
// is written once; files for pictures that are gone are removed.
func (p *Pipeline) keep() {
	if p.KeepDir == "" {
		return
	}
	if err := p.save(); err != nil {
		log.Printf("keep pictures: %v", err)
	}
}

func (p *Pipeline) save() error {
	if err := os.MkdirAll(p.KeepDir, 0o700); err != nil {
		return err
	}
	var index []kept
	files := map[string]bool{keepIndex: true, keepCounts: true}
	for _, t := range p.Mem.Keep() {
		k := memory.Key(t.Label)
		if t.Collected {
			index = append(index, kept{Kept: t})
			continue
		}
		img := p.images[k]
		if img == nil {
			continue
		}
		h := fnv.New64a()
		fmt.Fprintf(h, "%s|%d", k, t.Made.UnixNano())
		name := fmt.Sprintf("%x.png", h.Sum64())
		files[name] = true
		path := filepath.Join(p.KeepDir, name)
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			var buf bytes.Buffer
			if err := png.Encode(&buf, img); err != nil {
				return err
			}
			if err := writeFile(path, buf.Bytes()); err != nil {
				return err
			}
		}
		index = append(index, kept{t, p.styles[k], name})
	}
	b, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(p.KeepDir, keepIndex), b); err != nil {
		return err
	}
	if b, err = json.Marshal(p.Mem.Counts()); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(p.KeepDir, keepCounts), b); err != nil {
		return err
	}
	old, _ := os.ReadDir(p.KeepDir)
	for _, e := range old {
		if !files[e.Name()] && (strings.HasSuffix(e.Name(), ".png") || strings.HasSuffix(e.Name(), ".tmp")) {
			os.Remove(filepath.Join(p.KeepDir, e.Name()))
		}
	}
	return nil
}

// writeFile writes through a temporary file, so a crash never leaves half a file.
func writeFile(path string, b []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Restore loads the pictures KeepDir holds from an earlier run. A missing
// folder is no error: nothing was kept yet.
func (p *Pipeline) Restore() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.KeepDir == "" {
		return nil
	}
	// The daily count and the removed topics; a file from before they were
	// kept is simply not there.
	if b, err := os.ReadFile(filepath.Join(p.KeepDir, keepCounts)); err == nil {
		var c memory.Counts
		if err := json.Unmarshal(b, &c); err != nil {
			return fmt.Errorf("%s: %w", keepCounts, err)
		}
		p.Mem.RestoreCounts(c)
	}
	b, err := os.ReadFile(filepath.Join(p.KeepDir, keepIndex))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var index []kept
	if err := json.Unmarshal(b, &index); err != nil {
		return fmt.Errorf("%s: %w", keepIndex, err)
	}
	var back []memory.Kept
	for _, k := range index {
		if k.Collected {
			back = append(back, k.Kept)
			continue
		}
		f, err := os.Open(filepath.Join(p.KeepDir, filepath.Base(k.File)))
		if err != nil {
			continue // a lost picture file costs only that tile
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			continue
		}
		if p.images == nil {
			p.images, p.styles = map[string]image.Image{}, map[string]string{}
		}
		p.images[memory.Key(k.Label)] = img
		p.styles[memory.Key(k.Label)] = k.Style
		back = append(back, k.Kept)
	}
	p.Mem.Restore(back)
	log.Printf("kept pictures: %d back from the last run", len(back))
	return nil
}

// Forget deletes every kept picture and the index, for when keeping is
// turned off.
func Forget(dir string) error {
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if n := e.Name(); n == keepIndex || n == keepCounts || strings.HasSuffix(n, ".png") || strings.HasSuffix(n, ".tmp") {
			if err := os.Remove(filepath.Join(dir, n)); err != nil {
				return err
			}
		}
	}
	return nil
}
