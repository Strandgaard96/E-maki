package timeline

import (
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAddListAndReopen(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "timeline")
	s, err := Open(dir, Options{ThumbPx: 200, Quality: 80})
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 1024, 768))
	day := time.Date(2026, 11, 12, 20, 15, 0, 0, time.UTC)
	first, err := s.Add("garden gnome theft", "a gnome", "Comic", day, img)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Add("sourdough starter", "a jar", "Comic", day.Add(time.Hour), img)
	if err != nil {
		t.Fatal(err)
	}
	if first.W != 1024 || first.H != 768 {
		t.Errorf("size %dx%d", first.W, first.H)
	}
	again, err := Open(dir, Options{ThumbPx: 200, Quality: 80})
	if err != nil {
		t.Fatal(err)
	}
	list := again.List()
	if len(list) != 2 || list[0].ID != second.ID || list[1].ID != first.ID {
		t.Fatalf("list %+v, want newest first", list)
	}
	for _, thumb := range []bool{false, true} {
		if _, err := os.Stat(again.File(first.ID, thumb)); err != nil {
			t.Error(err)
		}
	}
	f, _ := os.Open(again.File(first.ID, true))
	cfg, err := jpeg.DecodeConfig(f)
	f.Close()
	if err != nil || cfg.Width != 200 || cfg.Height != 150 {
		t.Errorf("thumbnail %dx%d (%v), want 200x150", cfg.Width, cfg.Height, err)
	}
	if again.File("../settings", false) != "" || again.File("20260101-000000-00000000", false) != "" {
		t.Error("File answered for a picture that is not listed")
	}
}

func TestDeleteAll(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{}"), 0o600)
	s, _ := Open(dir, Options{ThumbPx: 480, Quality: 90})
	if _, err := s.Add("garden gnome theft", "a gnome", "Comic", time.Now(), image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteAll(); err != nil {
		t.Fatal(err)
	}
	if n := len(s.List()); n != 0 {
		t.Errorf("%d pictures left", n)
	}
	left, _ := os.ReadDir(dir)
	if len(left) != 1 || left[0].Name() != "settings.json" {
		t.Errorf("left behind: %v", left)
	}
}
