// Package config loads a hardware profile. Everything that differs between a
// laptop, a Pi 5 and a Jetson lives here, not in code.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"maps"
	"net"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/Strandgaard96/e-maki/internal/imagegen"
	"github.com/Strandgaard96/e-maki/internal/stt"
)

type Config struct {
	Profile string `json:"profile"`
	Listen  string `json:"listen"` // e.g. ":8090"

	// Speech-to-text sidecars: Detect does language detection and the
	// fallback transcription; ByLang overrides per language.
	WhisperURL    string            `json:"whisper_url"`
	WhisperByLang map[string]string `json:"whisper_by_lang"`

	LLMURL   string `json:"llm_url"`
	LLMModel string `json:"llm_model"` // the LLM_MODEL environment variable overrides it
	// reasoning ("none", "low", ...) is sent as reasoning_effort, e.g.
	// "none" so Qwen 3.5 answers without thinking.
	LLMReasoning string `json:"llm_reasoning"`

	SegmentSeconds int `json:"segment_seconds"` // audio per transcription call
	// An open microphone that sends no audio this long is reopened (a USB
	// mic that was unplugged). 0 is 5 seconds.
	MicTimeoutSeconds int `json:"mic_timeout_seconds"`

	// Picture styles of your own, offered next to the presets; one with a
	// preset's name replaces it.
	ImageStyles []imagegen.Style `json:"image_styles"`

	// How often System status asks whisper and the local topic model
	// whether they answer. 0 is 10 seconds.
	StatusCheckSeconds int `json:"status_check_seconds"`
	// What "Make a test picture" draws. Empty uses a built-in description.
	TestPicturePrompt string `json:"test_picture_prompt"`
	// The Timeline tab's saved pictures: thumbnail size (longest side, in
	// pixels), JPEG quality (1 to 100) and the height of a row of pictures
	// on a wide screen (in CSS pixels; a phone gets about half).
	TimelineThumbPx     int `json:"timeline_thumb_px"`
	TimelineJPEGQuality int `json:"timeline_jpeg_quality"`
	TimelineRowPx       int `json:"timeline_row_px"`

	// Where to listen for a network microphone (audio.NetMic), e.g.
	// ":10700". Empty is off. The NET_MIC_LISTEN environment variable
	// overrides it. The settings page then offers it as an input.
	NetMicListen string `json:"net_mic_listen"`

	// The keys the settings page can change. They sit at the top level of
	// the profile like the rest.
	Settings
}

// Settings are the options the settings page can change while E-maki runs.
// They are saved to their own file, which overrides the profile, so a
// profile stays a clean template. They hold no topics and no transcript text.
type Settings struct {
	Mode string `json:"mode"` // "diary" or "party"
	// When diary mode paints the topics it collected, like "07:00". Empty
	// is 07:00.
	DiaryAt string `json:"diary_at"`

	// The input to listen on, by the name the system gives it; empty (or a
	// device that is not plugged in) is the system default.
	MicDevice      string  `json:"mic_device"`
	MinSpeechDBFS  float64 `json:"min_speech_dbfs"` // quieter segments are skipped (Whisper invents text in silence)
	WindowSegments int     `json:"window_segments"` // transcript window kept in RAM
	ExtractEvery   int     `json:"extract_every"`   // segments between topic calls
	// Languages spoken in the house, as Whisper codes ("en", "da", "no").
	// One skips detection; with several, a guess outside the list is redone
	// in the first one. Empty lets Whisper pick any language.
	Languages []string `json:"languages"`
	// Optional topic LLM tuning. topic_prompt replaces topic.DefaultPrompt
	// (the privacy rules and JSON format are always added); temperature
	// defaults to 0.2.
	TopicPrompt    string   `json:"topic_prompt"`
	LLMTemperature *float64 `json:"llm_temperature"`
	// Who finds the topics: "local" (llm_url) or "external" (TOPIC_API_* in
	// .env). External sends the transcript window off the local network, so
	// it is never picked by default. Empty is "local".
	TopicProvider string `json:"topic_provider"`

	HalfLifeHours float64 `json:"half_life_hours"`
	MinConfidence float64 `json:"min_confidence"`
	DailyImageCap int     `json:"daily_image_cap"`
	// Sensitive categories ("health", "money", "relationship",
	// "confidential_work") never sent to the image API. Empty sends
	// everything; otherwise anything the LLM does not clearly mark as
	// "none" or an allowed category is held back too.
	BlockSensitive []string `json:"block_sensitive"`
	// Refuse an image prompt that repeats a name (person, place or brand)
	// heard in the transcript. See topic.CheckNames.
	BlockNames bool `json:"block_names"`
	// Words or phrases that never become a picture, checked against the
	// topic and its description (topic.CheckNever). Never logged.
	NeverPaint []string `json:"never_paint"`
	// Art style added to every image prompt: a preset's name (see
	// imagegen.Styles and image_styles) or a style of your own. Empty is
	// the first preset.
	ImageStyle string `json:"image_style"`
	// Who draws the pictures: "local", "gemini", "openai", "openrouter" or
	// "placeholder". Empty takes the first one set up in .env, local first.
	ImageProvider string `json:"image_provider"`

	// Layout: "collage" (heaviest topic biggest, tiles fade), "single" (the
	// newest topic only) or "rolling" (newest first, the oldest drops off).
	// Empty is "collage".
	Layout string `json:"layout"`
	Tiles  int    `json:"tiles"` // most tiles for collage and rolling, 1 to 9; 0 is 9
	// Least minutes between two new pictures going up; 0 puts each up at once.
	NewEveryMinutes int `json:"new_every_minutes"`
	// Collage only: the newest topic takes the big tile.
	NewestBig bool `json:"newest_big"`
	// Quiet hours "22:00" to "07:00": no new pictures go up in between, so
	// what was talked about appears at the end. Both empty is off.
	QuietFrom string `json:"quiet_from"`
	QuietTo   string `json:"quiet_to"`
	// Captions under the pictures are left out.
	HideCaptions bool `json:"hide_captions"`
	// Save the pictures on the frame and in the queue, with their topics and
	// descriptions, next to the settings file so the frame comes back after
	// a restart. Never what was said. Off keeps them in memory only.
	// Empty (null) is on.
	KeepPictures *bool `json:"keep_pictures"`
	// Save every new picture, with its topic, description, style and time,
	// next to the settings file for the Timeline tab. Never what was said.
	// Empty (null) is on.
	Timeline *bool `json:"timeline"`
	// The microphone is ignored: nothing is transcribed.
	Paused bool `json:"paused"`

	// What each of the frame's buttons does: "pause" (pause or resume
	// listening), "remove" (take the last picture off), "pin" (pin it or let
	// it fade again), "next" (put the next waiting picture up) or "" for
	// nothing. A button left out does its default (DefaultButtons).
	PanelButtons map[string]string `json:"panel_buttons"`

	// The panel's resolution as its maker gives it, landscape: 1600x1200
	// for the 13.3" Inky Impression, 800x480 for the 7.3", 600x448 for the
	// 5.7" and 640x400 for the 4". Zero is the 13.3". panel_rotation turns
	// the page to portrait.
	PanelWidth  int `json:"panel_width"`
	PanelHeight int `json:"panel_height"`

	PanelRotation int `json:"panel_rotation"`
	PanelPoll     int `json:"panel_poll_minutes"`
}

// Buttons are the frame's buttons, and ButtonActions what they can do.
var (
	Buttons        = []string{"A", "B", "C", "D"}
	ButtonActions  = []string{"pause", "remove", "pin", "next", ""}
	DefaultButtons = map[string]string{"A": "pause", "B": "remove", "C": "pin", "D": "next"}
)

// Button is what a button does with these settings.
func (s Settings) Button(b string) string {
	if a, ok := s.PanelButtons[b]; ok {
		return a
	}
	return DefaultButtons[b]
}

// Categories are the sensitive categories block_sensitive can name.
var Categories = []string{"health", "money", "relationship", "confidential_work"}

// Load reads and checks a profile. A key it does not know is an error: a
// misspelt block_sensitive must not silently send everything.
func Load(path string) (Config, error) {
	var c Config
	b, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return c, fmt.Errorf("%s: %w", path, err)
	}
	c.defaults()
	if err := c.validate(); err != nil {
		return c, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

func (c Config) validate() error {
	for _, e := range []struct {
		ok  bool
		msg string
	}{
		{c.Listen != "", "listen is empty"},
		{c.Mode == "diary" || c.Mode == "party", `mode must be "diary" or "party"`},
		{c.WhisperURL != "" && c.LLMURL != "", "whisper_url and llm_url are needed"},
		{c.SegmentSeconds > 0, "segment_seconds must be above 0"},
		{c.MicTimeoutSeconds >= 0, "mic_timeout_seconds must not be negative"},
		{c.StatusCheckSeconds > 0, "status_check_seconds must be above 0"},
		{c.TimelineThumbPx >= 64 && c.TimelineThumbPx <= 2048, "timeline_thumb_px must be 64 to 2048"},
		{c.TimelineJPEGQuality >= 1 && c.TimelineJPEGQuality <= 100, "timeline_jpeg_quality must be 1 to 100"},
		{c.TimelineRowPx >= 60 && c.TimelineRowPx <= 800, "timeline_row_px must be 60 to 800"},
		{c.WindowSegments > 0, "window_segments must be above 0"},
		{c.ExtractEvery > 0, "extract_every must be above 0"},
		{c.HalfLifeHours > 0, "half_life_hours must be above 0"},
		{c.MinConfidence >= 0 && c.MinConfidence <= 1, "min_confidence must be between 0 and 1"},
		{c.MinSpeechDBFS >= -120 && c.MinSpeechDBFS <= 0, "min_speech_dbfs must be between -120 and 0"},
		{c.WindowSegments <= 200 && c.ExtractEvery <= 200, "window_segments and extract_every must be at most 200"},
		{c.LLMTemperature == nil || (*c.LLMTemperature >= 0 && *c.LLMTemperature <= 2), "llm_temperature must be between 0 and 2"},
		{c.TopicProvider == "" || c.TopicProvider == "local" || c.TopicProvider == "external", `topic_provider must be "local" or "external"`},
		{len(c.TopicPrompt) <= 8000, "topic_prompt must be at most 8000 characters"},
		{len(c.NeverPaint) <= 200, "never_paint holds at most 200 entries"},
		{c.DailyImageCap >= 0, "daily_image_cap must not be negative"},
		{c.PanelRotation%90 == 0, "panel_rotation must be a multiple of 90"},
		{c.PanelWidth >= 100 && c.PanelHeight >= 100 && c.PanelWidth <= 4096 && c.PanelHeight <= 4096, "panel_width and panel_height must be between 100 and 4096"},
		{c.PanelWidth%2 == 0, "panel_width must be even"},
		{c.PanelPoll >= 0 && c.PanelPoll <= 255, "panel_poll_minutes must be between 0 and 255"},
		{c.Layout == "collage" || c.Layout == "single" || c.Layout == "rolling", `layout must be "collage", "single" or "rolling"`},
		{c.Tiles >= 1 && c.Tiles <= 9, "tiles must be between 1 and 9"},
		{c.NewEveryMinutes >= 0 && c.NewEveryMinutes <= 24*60, "new_every_minutes must be between 0 and 1440"},
		{(c.QuietFrom == "") == (c.QuietTo == ""), "set both quiet_from and quiet_to, or neither"},
	} {
		if !e.ok {
			return fmt.Errorf("%s", e.msg)
		}
	}
	if _, ok := Clock(c.DiaryAt); !ok {
		return fmt.Errorf("diary_at: %q is not a time like 07:00", c.DiaryAt)
	}
	for _, t := range []string{c.QuietFrom, c.QuietTo} {
		if _, ok := Clock(t); t != "" && !ok {
			return fmt.Errorf("quiet hours: %q is not a time like 22:00", t)
		}
	}
	for _, st := range c.ImageStyles {
		if strings.TrimSpace(st.Name) == "" || strings.TrimSpace(st.Prompt) == "" {
			return fmt.Errorf("image_styles: each style needs a name and a prompt")
		}
	}
	for b, a := range c.PanelButtons {
		if !slices.Contains(Buttons, b) || !slices.Contains(ButtonActions, a) {
			return fmt.Errorf("panel_buttons: %q: %q is not a button (A to D) and one of pause, remove, pin, next or \"\"", b, a)
		}
	}
	for _, w := range c.NeverPaint {
		if len(w) > 100 {
			return fmt.Errorf("never_paint: %d characters is too long for one entry, at most 100", len(w))
		}
	}
	for i, l := range c.Languages {
		if !stt.Known(l) || slices.Index(c.Languages, l) != i {
			return fmt.Errorf("languages: %q is not a Whisper language code like \"da\", or is listed twice", l)
		}
	}
	for _, b := range c.BlockSensitive {
		if !slices.Contains(Categories, b) {
			return fmt.Errorf("block_sensitive: unknown category %q", b)
		}
	}
	return nil
}

// Local reports whether a sidecar URL stays on this machine or the local
// network: a loopback or private address, or a name without a public domain
// (a compose service, "jetson.local"). Audio and transcripts go to these
// URLs, so anything else deserves a warning.
func Local(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
	}
	for _, suffix := range []string{".local", ".lan", ".internal", ".home.arpa"} {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return host != "" && !strings.Contains(host, ".")
}

// Clock parses "22:00" to minutes after midnight.
func Clock(s string) (int, bool) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, false
	}
	return t.Hour()*60 + t.Minute(), true
}

// MinMentions is the "seen twice" rule: diary mode waits for a topic to come
// back, party mode shows it straight away.
func (c Config) MinMentions() int {
	if c.Mode == "party" {
		return 1
	}
	return 2
}

// KeepPicturesOn reports whether the frame's pictures are kept over a restart.
func (s Settings) KeepPicturesOn() bool { return s.KeepPictures == nil || *s.KeepPictures }

// TimelineOn reports whether new pictures go to the timeline.
func (s Settings) TimelineOn() bool { return s.Timeline == nil || *s.Timeline }

// PageSize is the page as the viewer sees it: the panel's resolution,
// turned on its side when the frame hangs rotated a quarter turn.
func (c Config) PageSize() image.Point {
	if c.PanelRotation%180 != 0 {
		return image.Pt(c.PanelHeight, c.PanelWidth)
	}
	return image.Pt(c.PanelWidth, c.PanelHeight)
}

func (c Config) HalfLife() time.Duration { return time.Duration(c.HalfLifeHours * float64(time.Hour)) }

// With returns the config with s applied, or an error if s is out of range.
func (c Config) With(s Settings) (Config, error) {
	s.ImageStyle = strings.TrimSpace(s.ImageStyle)
	s.TopicPrompt = strings.TrimSpace(s.TopicPrompt)
	var never []string
	for _, w := range s.NeverPaint {
		if w = strings.TrimSpace(w); w != "" {
			never = append(never, w)
		}
	}
	s.NeverPaint = never
	c.Settings = s
	c.defaults()
	return c, c.validate()
}

func (c *Config) defaults() {
	if c.Layout == "" {
		c.Layout = "collage"
	}
	if c.Tiles == 0 {
		c.Tiles = 9
	}
	if c.DiaryAt == "" {
		c.DiaryAt = "07:00"
	}
	if c.MicTimeoutSeconds == 0 {
		c.MicTimeoutSeconds = 5
	}
	if c.StatusCheckSeconds == 0 {
		c.StatusCheckSeconds = 10
	}
	if c.TimelineThumbPx == 0 {
		c.TimelineThumbPx = 480
	}
	if c.TimelineJPEGQuality == 0 {
		c.TimelineJPEGQuality = 90
	}
	if c.TimelineRowPx == 0 {
		c.TimelineRowPx = 220
	}
	buttons := maps.Clone(DefaultButtons)
	maps.Copy(buttons, c.PanelButtons)
	c.PanelButtons = buttons
	if c.PanelWidth == 0 && c.PanelHeight == 0 {
		c.PanelWidth, c.PanelHeight = 1600, 1200
	}
}

// LoadSettings applies a saved settings file to c. A missing file is no
// error: nothing has been changed on the web page yet.
func LoadSettings(path string, c Config) (Config, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	s := c.Settings
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return c, fmt.Errorf("%s: %w", path, err)
	}
	c, err = c.With(s)
	if err != nil {
		return c, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

// SaveSettings writes s to path through a temporary file, so a crash never
// leaves half a file.
func SaveSettings(path string, s Settings) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
