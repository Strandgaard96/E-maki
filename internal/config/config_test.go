package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every shipped profile must load.
func TestShippedProfiles(t *testing.T) {
	files, _ := filepath.Glob("../../configs/*.json")
	if len(files) == 0 {
		t.Fatal("no profiles found")
	}
	for _, f := range files {
		if _, err := Load(f); err != nil {
			t.Error(err)
		}
	}
}

func TestLoadRejectsMistakes(t *testing.T) {
	good, err := os.ReadFile("../../configs/docker.json")
	if err != nil {
		t.Fatal(err)
	}
	for name, edit := range map[string][2]string{
		"misspelt key":     {`"block_sensitive"`, `"block_sensitve"`},
		"zero interval":    {`"extract_every": 4`, `"extract_every": 0`},
		"unknown category": {`"health"`, `"medical"`},
		"unknown mode":     {`"party"`, `"fiesta"`},
		"unknown topics":   {`"mode"`, `"topic_provider": "cloud", "mode"`},
		"language name":    {`"mode"`, `"languages": ["Danish"], "mode"`},
		"unknown language": {`"mode"`, `"languages": ["xx"], "mode"`},
		"language twice":   {`"mode"`, `"languages": ["da", "da"], "mode"`},
	} {
		bad := strings.Replace(string(good), edit[0], edit[1], 1)
		if bad == string(good) {
			t.Fatalf("%s: nothing replaced", name)
		}
		path := filepath.Join(t.TempDir(), "c.json")
		os.WriteFile(path, []byte(bad), 0o600)
		if _, err := Load(path); err == nil {
			t.Errorf("%s: loaded", name)
		}
	}
}

func TestLocal(t *testing.T) {
	for u, want := range map[string]bool{
		"http://localhost:8081":       true,
		"http://whisper:8081":         true,
		"http://192.168.1.20:11434":   true,
		"http://[::1]:11434":          true,
		"http://jetson.local:11434":   true,
		"https://api.example.com/v1":  false,
		"http://8.8.8.8:11434":        false,
		"not a url at all, no scheme": false,
	} {
		if got := Local(u); got != want {
			t.Errorf("Local(%q) = %v, want %v", u, got, want)
		}
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	c, err := Load("../../configs/docker.json")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "settings.json")
	if got, err := LoadSettings(path, c); err != nil || got.Mode != c.Mode {
		t.Fatalf("no file: %v, mode %q", err, got.Mode)
	}
	s := c.Settings
	s.Mode, s.ImageStyle, s.PanelRotation = "diary", "  linocut  ", 90
	if err := SaveSettings(path, s); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSettings(path, c)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != "diary" || got.ImageStyle != "linocut" || got.PanelRotation != 90 || got.WhisperURL != c.WhisperURL {
		t.Fatalf("loaded %+v", got.Settings)
	}
	s.Mode = "fiesta"
	if _, err := c.With(s); err == nil {
		t.Error("With accepted mode fiesta")
	}
	os.WriteFile(path, []byte(`{"whisper_url": "http://example.com"}`), 0o600)
	if _, err := LoadSettings(path, c); err == nil {
		t.Error("a settings file may not change the sidecars")
	}
}

func TestLayoutSettings(t *testing.T) {
	c, err := Load("../../configs/docker.json")
	if err != nil {
		t.Fatal(err)
	}
	if c.Layout != "collage" || c.Tiles != 9 {
		t.Fatalf("defaults: layout %q, tiles %d", c.Layout, c.Tiles)
	}
	for name, edit := range map[string]func(*Settings){
		"layout":     func(s *Settings) { s.Layout = "mosaic" },
		"tiles":      func(s *Settings) { s.Tiles = 12 },
		"half quiet": func(s *Settings) { s.QuietFrom = "22:00" },
		"bad clock":  func(s *Settings) { s.QuietFrom, s.QuietTo = "25:00", "07:00" },
		"every":      func(s *Settings) { s.NewEveryMinutes = -1 },
	} {
		s := c.Settings
		edit(&s)
		if _, err := c.With(s); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	s := c.Settings
	s.Layout, s.Tiles, s.QuietFrom, s.QuietTo = "rolling", 4, "22:00", "07:00"
	if _, err := c.With(s); err != nil {
		t.Error(err)
	}
}

func TestDiaryAt(t *testing.T) {
	c, err := Load("../../configs/jetson.json")
	if err != nil {
		t.Fatal(err)
	}
	if c.DiaryAt != "07:00" {
		t.Fatalf("diary_at %q, want the 07:00 default", c.DiaryAt)
	}
	s := c.Settings
	s.DiaryAt = "7am"
	if _, err := c.With(s); err == nil {
		t.Fatal("diary_at 7am accepted")
	}
}

func TestPanelButtons(t *testing.T) {
	c, err := Load("../../configs/docker.json")
	if err != nil {
		t.Fatal(err)
	}
	if c.Button("A") != "pause" || c.Button("D") != "next" {
		t.Fatalf("defaults: %v", c.PanelButtons)
	}
	s := c.Settings
	s.PanelButtons = map[string]string{"A": "", "B": "pause"}
	c2, err := c.With(s)
	if err != nil || c2.Button("A") != "" || c2.Button("B") != "pause" || c2.Button("C") != "" {
		t.Fatalf("remapped: %v, %v", c2.PanelButtons, err)
	}
	s.PanelButtons = map[string]string{}
	if c3, err := c.With(s); err != nil || len(c3.PanelButtons) != 0 || c3.Button("A") != "" {
		t.Fatalf("no buttons: %v, %v", c3.PanelButtons, err)
	}
	// A settings file with fewer buttons replaces the profile's, not merges.
	path := t.TempDir() + "/settings.json"
	s.PanelButtons = map[string]string{"A": "next"}
	if err := SaveSettings(path, s); err != nil {
		t.Fatal(err)
	}
	if c4, err := LoadSettings(path, c); err != nil || len(c4.PanelButtons) != 1 || c4.Button("A") != "next" {
		t.Fatalf("loaded: %v, %v", c4.PanelButtons, err)
	}
	if DefaultButtons["A"] != "pause" {
		t.Fatal("With changed the defaults")
	}
	for _, bad := range []map[string]string{{"I": "pause"}, {"A": "dance"}} {
		s.PanelButtons = bad
		if _, err := c.With(s); err == nil {
			t.Errorf("%v: no error", bad)
		}
	}
}

func TestKeepPicturesDefaultOn(t *testing.T) {
	if !(Settings{}).KeepPicturesOn() {
		t.Error("unset keep_pictures should be on")
	}
	off := false
	if (Settings{KeepPictures: &off}).KeepPicturesOn() {
		t.Error("keep_pictures false should be off")
	}
}
