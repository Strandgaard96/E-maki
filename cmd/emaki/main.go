// Command emaki listens to a room and paints what it talks about.
//
// It listens on a microphone (-mic), or runs on recorded input: a WAV file
// through the speech-to-text and LLM sidecars, or a synthetic text script
// through fakes. Either way it serves the page with Fugleramme's panel
// protocol, and the settings page.
package main

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	_ "time/tzdata" // quiet hours and captions use TZ, also in a slim container

	"github.com/Strandgaard96/e-maki/internal/audio"
	"github.com/Strandgaard96/e-maki/internal/config"
	"github.com/Strandgaard96/e-maki/internal/health"
	"github.com/Strandgaard96/e-maki/internal/imagegen"
	"github.com/Strandgaard96/e-maki/internal/memory"
	"github.com/Strandgaard96/e-maki/internal/panel"
	"github.com/Strandgaard96/e-maki/internal/pipeline"
	"github.com/Strandgaard96/e-maki/internal/stt"
	"github.com/Strandgaard96/e-maki/internal/timeline"
	"github.com/Strandgaard96/e-maki/internal/topic"
	"github.com/Strandgaard96/e-maki/internal/web"
)

func main() {
	cfgPath := flag.String("config", "configs/laptop.json", "hardware profile")
	micFlag := flag.Bool("mic", false, "listen on the default microphone (needs the sidecars)")
	wav := flag.String("wav", "", "16 kHz mono WAV to transcribe (needs the sidecars)")
	script := flag.String("script", "", "synthetic transcript, one segment per line (runs offline with fakes)")
	settingsPath := flag.String("settings", "", "file that keeps what the settings page changes (empty: changes last until restart)")
	flag.Parse()
	// System > Logs shows the last lines from memory; stderr gets them too.
	logs := web.NewLogRing(1000)
	log.SetOutput(io.MultiWriter(os.Stderr, logs))

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatal(err)
	}
	imagegen.AddStyles(cfg.ImageStyles)
	if *settingsPath != "" {
		if cfg, err = config.LoadSettings(*settingsPath, cfg); err != nil {
			log.Fatal(err)
		}
	}
	// One place (.env) picks the model for both Ollama's pull and E-maki.
	if m := strings.TrimSpace(os.Getenv("LLM_MODEL")); m != "" {
		cfg.LLMModel = m
	}
	// WHISPER_URL_DA=http://whisper-da:8081 adds a Danish server to
	// whisper_by_lang (the Danish/Norwegian compose overlay sets these).
	for _, kv := range os.Environ() {
		k, v, _ := strings.Cut(kv, "=")
		if lang, ok := strings.CutPrefix(k, "WHISPER_URL_"); ok && strings.TrimSpace(v) != "" {
			if cfg.WhisperByLang == nil {
				cfg.WhisperByLang = map[string]string{}
			}
			cfg.WhisperByLang[strings.ToLower(lang)] = strings.TrimSpace(v)
		}
	}
	// The external topic model, if set up in .env. Only topic_provider
	// "external" uses it.
	ext := topic.ExternalFromEnv(os.Getenv)
	// NO_LOCAL_TOPICS=1 leaves Ollama out of the compose setup.
	local := strings.TrimSpace(os.Getenv("NO_LOCAL_TOPICS")) == ""
	if cfg.TopicProvider == "external" && !ext.Ready() {
		log.Printf("topics: the external topic model is not set up: %s. Using the local one until then", topic.ExternalSetup)
	}
	logTopics(cfg, ext, local)
	// Whether the sidecars and the image provider answer, for the page.
	tracker := &health.Tracker{}
	// Audio and transcripts go to the sidecars, so they must be local.
	sidecars := []string{cfg.WhisperURL, cfg.LLMURL}
	for _, u := range cfg.WhisperByLang {
		sidecars = append(sidecars, u)
	}
	for _, u := range sidecars {
		if !config.Local(u) {
			log.Printf("WARNING: %s does not look like a local address: audio or transcripts sent there leave your network", u)
		}
	}
	if v, ok := os.LookupEnv("NET_MIC_LISTEN"); ok {
		cfg.NetMicListen = strings.TrimSpace(v)
	}
	// PANEL_TOKEN moves the panel under /<token>/ (see panel.Server).
	token := strings.TrimSpace(os.Getenv("PANEL_TOKEN"))
	if !regexp.MustCompile(`^[A-Za-z0-9_-]*$`).MatchString(token) {
		log.Fatal("PANEL_TOKEN may only hold letters, digits, - and _")
	}
	srv := &panel.Server{Token: token, Rotation: cfg.PanelRotation, Poll: cfg.PanelPoll}
	// The image providers set up in .env (keys and addresses come from the
	// environment, never a file in the repo). The profile or the settings
	// page picks one with image_provider.
	choices := imagegen.FromEnv(os.Getenv)
	if u := os.Getenv("LOCAL_IMAGE_URL"); u != "" && !config.Local(u) {
		log.Printf("WARNING: LOCAL_IMAGE_URL %s does not look like a local address: picture descriptions sent there leave your network", u)
	}
	pick := func(c config.Config) imagegen.Choice {
		if err := imagegen.Check(choices, c.ImageProvider); err != nil {
			log.Printf("images: %v. Using placeholders until then", err)
		}
		ch, _ := imagegen.Pick(choices, c.ImageProvider)
		return ch
	}
	images := func(c config.Config) imagegen.Provider {
		return health.Provider{P: pick(c).New(c.ImageStyle), Tracker: tracker}
	}
	memSettings := func(c config.Config) memory.Settings {
		tiles := c.Tiles
		if c.Layout == "single" {
			tiles = 1
		}
		from, _ := config.Clock(c.QuietFrom)
		to, _ := config.Clock(c.QuietTo)
		diaryAt, _ := config.Clock(c.DiaryAt)
		every := time.Duration(c.NewEveryMinutes) * time.Minute
		if c.Mode == "diary" {
			every = 0 // the day's pictures go up together
		}
		return memory.Settings{
			HalfLife: c.HalfLife(), MinConfidence: c.MinConfidence, MinMentions: c.MinMentions(),
			DailyImageCap: c.DailyImageCap, MaxTiles: tiles, MinWeight: 0.1, BlockFor: 7 * 24 * time.Hour,
			Newest: c.Layout != "collage", Every: every, RotateEvery: time.Duration(c.RotateMinutes) * time.Minute,
			NewestBig: c.Arrange != "even", QuietFrom: from, QuietTo: to,
			Diary: c.Mode == "diary", DiaryAt: diaryAt,
		}
	}
	p := &pipeline.Pipeline{
		Mem:            memory.New(memSettings(cfg)),
		Extractor:      extractor(cfg, ext, local, tracker),
		Images:         images(cfg),
		Panel:          srv,
		WindowSegments: cfg.WindowSegments,
		ExtractEvery:   cfg.ExtractEvery,
		BlockSensitive: cfg.BlockSensitive,
		BlockNames:     cfg.BlockNames,
		NeverPaint:     cfg.NeverPaint,
		HideCaptions:   cfg.HideCaptions,
		Look:           cfg.Look(),
		Style:          styleName(cfg.ImageStyle),
		Size:           cfg.PageSize(),
		Now:            time.Now,
	}
	// Pictures are kept next to the settings file, and only when asked.
	keepDir := func(c config.Config) string {
		if !c.KeepPicturesOn() || *settingsPath == "" {
			return ""
		}
		return filepath.Join(filepath.Dir(*settingsPath), "pictures")
	}
	// The pictures for Pictures > History live next to the settings file too.
	var pics *timeline.Store
	if *settingsPath != "" {
		if pics, err = timeline.Open(filepath.Join(filepath.Dir(*settingsPath), "timeline"),
			timeline.Options{ThumbPx: cfg.TimelineThumbPx, Quality: cfg.TimelineJPEGQuality}); err != nil {
			log.Fatal(err)
		}
	}
	timelineFor := func(c config.Config) *timeline.Store {
		if !c.TimelineOn() {
			return nil
		}
		return pics
	}
	options := func(c config.Config) pipeline.Options {
		ex := extractor(c, ext, local, tracker)
		if *script != "" {
			ex = topic.Fake{}
		}
		return pipeline.Options{Extractor: ex, WindowSegments: c.WindowSegments, ExtractEvery: c.ExtractEvery,
			Images: images(c), BlockSensitive: c.BlockSensitive, BlockNames: c.BlockNames, NeverPaint: c.NeverPaint,
			HideCaptions: c.HideCaptions, Look: c.Look(), Style: styleName(c.ImageStyle), KeepDir: keepDir(c), Timeline: timelineFor(c), Paused: c.Paused,
			Size: c.PageSize()}
	}
	p.Timeline = timelineFor(cfg)
	if cfg.Paused {
		p.Configure(options(cfg), memSettings(cfg))
	}
	// live is the config as the settings page last left it.
	var live atomic.Pointer[config.Config]
	live.Store(&cfg)
	log.Printf("images: %s (daily cap %d)", pick(cfg).Name, cfg.DailyImageCap)

	// The microphone as the settings page shows it, and a nudge to reopen
	// it when another one is picked there, or to close or open it when
	// listening is paused or resumed.
	mic := &micState{network: cfg.NetMicListen != ""}
	micChanged := make(chan struct{}, 1)

	// The settings page changes these while E-maki runs.
	page := &web.Server{
		Token: token, Pipeline: p, Panel: srv, Logs: logs, Images: choices, Health: tracker, TestPrompt: cfg.TestPicturePrompt, TestPicturesPerDay: cfg.TestPicturesPerDay,
		Timeline: pics, TimelineRow: cfg.TimelineRowPx,
		Status: web.Status{Profile: cfg.Profile, Topics: cfg.LLMModel, NoLocalTopics: !local, LLMURL: cfg.LLMURL, TopicAPI: externalName(ext), Speech: cfg.WhisperURL,
			ByLang: cfg.WhisperByLang, Segment: cfg.SegmentSeconds,
			Locked: token != "", Saved: *settingsPath != "", Started: time.Now()},
		Apply: func(s config.Settings) (config.Settings, error) {
			c, err := cfg.With(s)
			if err != nil {
				return s, err
			}
			// Only a new choice is refused; one that stopped working since the
			// last start already falls back to placeholders.
			if c.ImageProvider != live.Load().ImageProvider {
				if err := imagegen.Check(choices, c.ImageProvider); err != nil {
					return s, err
				}
			}
			newTopics := c.TopicProvider != live.Load().TopicProvider
			if newTopics && c.TopicProvider == "external" && !ext.Ready() {
				return s, fmt.Errorf("the external topic model is not set up: %s", topic.ExternalSetup)
			}
			if newTopics && c.TopicProvider != "external" && !local {
				return s, errNoLocalTopics
			}
			if err := srv.SetFrame(c.PanelRotation, c.PanelPoll); err != nil {
				return s, err
			}
			reopen := c.MicDevice != live.Load().MicDevice || c.Paused != live.Load().Paused
			now, _ := imagegen.Pick(choices, c.ImageProvider)
			was, _ := imagegen.Pick(choices, live.Load().ImageProvider)
			if now.ID != was.ID {
				tracker.Forget(health.Images)
			}
			live.Store(&c)
			if reopen { // after Store, so the microphone loop reads the new settings
				select {
				case micChanged <- struct{}{}:
				default:
				}
			}
			if newTopics {
				tracker.Forget(health.Topics)
				logTopics(c, ext, local)
			}
			return c.Settings, p.Configure(options(c), memSettings(c))
		},
		Mic: func() web.Mic { return mic.status(*micFlag, p.Paused()) },
		Draft: func(s config.Settings) (image.Image, error) {
			c, err := cfg.With(s)
			if err != nil {
				return nil, err
			}
			return p.Draft(memSettings(c), c.HideCaptions, c.Look(), c.PageSize()), nil
		},
	}
	// The settings page's optional password lives next to the settings.
	authPath := ""
	if *settingsPath != "" {
		page.Save = func(s config.Settings) error { return config.SaveSettings(*settingsPath, s) }
		authPath = filepath.Join(filepath.Dir(*settingsPath), "password.json")
	}
	if page.Auth, err = web.LoadAuth(authPath); err != nil {
		log.Fatal(err)
	}
	page.SetSettings(cfg.Settings)
	srv.OnPress = page.Press

	// The log shows where the page is, without the token itself.
	path := "/frame.png"
	if token != "" {
		path = "/<PANEL_TOKEN>/frame.png"
	}
	p.FrameURL = "http://localhost" + cfg.Listen + path
	if !strings.HasPrefix(cfg.Listen, ":") {
		p.FrameURL = "http://" + cfg.Listen + path
	}

	p.KeepDir = keepDir(cfg)
	if err := p.Restore(); err != nil {
		log.Printf("kept pictures: %v", err)
	}
	if err := p.Start(); err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	srv.Register(mux)
	page.Register(mux)
	// Timeouts, so a client that stalls cannot hold connections open.
	// EMAKI_HOSTS=frame.example.com,brain.tailnet.ts.net lets the page answer
	// to names that are not plainly local (see web.Protect).
	var hosts []string
	for _, h := range strings.Split(os.Getenv("EMAKI_HOSTS"), ",") {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			hosts = append(hosts, h)
		}
	}
	hs := &http.Server{Addr: cfg.Listen, Handler: web.Protect(mux, hosts), ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout: 30 * time.Second, WriteTimeout: 2 * time.Minute, IdleTimeout: 2 * time.Minute}
	go func() { log.Fatal(hs.ListenAndServe()) }()
	// The page is up before the models are: it shows them as loading
	// until they answer. An external topic model is not probed.
	go tracker.Watch(context.Background(), time.Duration(cfg.StatusCheckSeconds)*time.Second, func() map[string]string {
		t := map[string]string{health.Speech: strings.TrimRight(cfg.WhisperURL, "/") + "/health"}
		if c := live.Load(); local && (c.TopicProvider != "external" || !ext.Ready()) {
			t[health.Topics] = strings.TrimRight(c.LLMURL, "/") + "/v1/models"
		}
		return t
	})
	// New topics waiting for their turn (new_every_minutes, quiet hours),
	// and diary mode's daily painting.
	go func() {
		for range time.Tick(30 * time.Second) {
			p.Tick()
			p.Edition(context.Background())
		}
	}()
	mode := cfg.Mode
	if mode == "party" {
		mode = "continuous" // its name on the settings page
	}
	log.Printf("profile %s, mode %s, serving the panel on %s", cfg.Profile, mode, cfg.Listen)
	log.Printf("settings page: %s", strings.TrimSuffix(p.FrameURL, "frame.png"))

	ctx := context.Background()
	switch {
	case *script != "":
		p.Extractor = topic.Fake{}
		f, err := os.Open(*script)
		if err != nil {
			log.Fatal(err)
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if err := p.Feed(ctx, sc.Text()); err != nil {
				log.Print(err)
			}
		}
		f.Close()
	case *wav != "":
		pcm, err := audio.ReadWAV(*wav)
		if err != nil {
			log.Fatal(err)
		}
		step := cfg.SegmentSeconds * audio.SampleRate
		for i := 0; i < len(pcm); i += step {
			feed(ctx, p, hear(ctx, live.Load().MinSpeechDBFS, live.Load().Languages, p, router(cfg, tracker), pcm[i:min(i+step, len(pcm))]))
		}
		pcm = nil
	case *micFlag:
		segs := make(chan []int16, 2)
		go listen(ctx, &live, mic, micChanged, cfg.SegmentSeconds*audio.SampleRate, segs, cfg.NetMicListen, token)
		log.Printf("listening on a microphone, %d s segments", cfg.SegmentSeconds)
		// Topic calls take as long as a segment, so they run beside
		// transcription: texts wait in RAM instead of the mic dropping audio.
		texts := make(chan string, 16)
		go func() {
			for t := range texts {
				feed(ctx, p, t)
			}
		}()
		r := router(cfg, tracker)
		for seg := range segs {
			if p.Paused() {
				continue // paused on the settings page: dropped unheard
			}
			select {
			case texts <- hear(ctx, live.Load().MinSpeechDBFS, live.Load().Languages, p, r, seg):
			default:
				log.Print("topic queue full: dropped a segment's text")
				p.Dropped()
			}
		}
	default:
		log.Fatal("give -mic, -wav or -script")
	}
	log.Print("input done; still serving the panel (Ctrl-C to stop)")
	select {}
}

func extractor(c config.Config, ext topic.External, local bool, t *health.Tracker) topic.Extractor {
	if c.TopicProvider == "external" && ext.Ready() {
		return health.Extractor{E: ext.Extractor(c.TopicPrompt, c.LLMTemperature), Tracker: t, Name: ext.Name()}
	}
	if !local {
		return health.Extractor{E: noTopics{}, Tracker: t, Name: "none"}
	}
	return health.Extractor{E: topic.OpenAI{URL: c.LLMURL, Model: c.LLMModel, Prompt: c.TopicPrompt,
		Temperature: c.LLMTemperature, Reasoning: c.LLMReasoning}, Tracker: t, Name: c.LLMModel + " at " + c.LLMURL}
}

// errNoLocalTopics is why no topics are found when the local topic model is
// off and the external one is not picked. The external one is never picked
// for the user.
var errNoLocalTopics = errors.New("the local topic model is off (NO_LOCAL_TOPICS in .env); pick the external one on the settings page")

type noTopics struct{}

func (noTopics) Extract(context.Context, string, []string) (topic.Result, error) {
	return topic.Result{}, errNoLocalTopics
}

// logTopics says which topic model is in use, with a warning when the
// transcript leaves the house.
func logTopics(c config.Config, ext topic.External, local bool) {
	if c.TopicProvider == "external" && ext.Ready() {
		log.Printf("topics: %s. WARNING: the transcript window is sent to this external service", ext.Name())
		return
	}
	if !local {
		log.Printf("topics: none. %v", errNoLocalTopics)
		return
	}
	log.Printf("topics: %s at %s", c.LLMModel, c.LLMURL)
}

// externalName is "" when the external topic model is not set up.
func externalName(ext topic.External) string {
	if !ext.Ready() {
		return ""
	}
	return ext.Name()
}

func router(cfg config.Config, t *health.Tracker) stt.Router {
	whisper := func(url string) stt.Transcriber { return health.Transcriber{T: stt.Whisper{URL: url}, Tracker: t} }
	r := stt.Router{Detect: whisper(cfg.WhisperURL), ByLang: map[string]stt.Transcriber{}}
	for lang, url := range cfg.WhisperByLang {
		r.ByLang[lang] = whisper(url)
	}
	return r
}

// hear transcribes one segment; quiet segments give "". Only the level and
// language are logged, never the text.
func hear(ctx context.Context, minDBFS float64, langs []string, p *pipeline.Pipeline, r stt.Router, pcm []int16) string {
	level := audio.LoudestDBFS(pcm)
	if level < minDBFS {
		log.Printf("segment at %.0f dBFS: quiet, skipped", level)
		p.Heard(level, "", true)
		return ""
	}
	start := time.Now()
	seg, err := r.In(ctx, pcm, langs)
	if err != nil {
		log.Printf("transcribe: %v", err)
		return ""
	}
	if seg.Text == "" {
		// Whisper's voice detection found no speech. Counted with the quiet
		// segments; the language it reports then means nothing.
		log.Printf("segment at %.0f dBFS: no speech found in %.1fs", level, time.Since(start).Seconds())
		p.Heard(level, "", true)
		return ""
	}
	log.Printf("segment at %.0f dBFS: %s, %d words, transcribed in %.1fs",
		level, seg.Lang, len(strings.Fields(seg.Text)), time.Since(start).Seconds())
	p.Heard(level, seg.Lang, false)
	return seg.Text
}

func feed(ctx context.Context, p *pipeline.Pipeline, text string) {
	if err := p.Feed(ctx, text); err != nil {
		log.Printf("topic: %v", err)
	}
}

// micState is what the settings page is told about the microphone: which
// one is open and how loud it is, never audio.
type micState struct {
	mu      sync.Mutex
	active  string
	open    bool
	err     string
	level   float64
	devices []audio.Device
	listErr string
	listed  time.Time
	network bool // a network microphone is set up (net_mic_listen)
}

func (m *micState) set(f func(*micState)) {
	m.mu.Lock()
	f(m)
	m.mu.Unlock()
}

// status lists the inputs (looked up at most every few seconds) and says
// which one is open. The level is left out while paused: the page should
// not look as if it listens.
func (m *micState) status(live, paused bool) web.Mic {
	m.mu.Lock()
	defer m.mu.Unlock()
	if time.Since(m.listed) > 5*time.Second {
		m.devices, m.listErr, m.listed = nil, "", time.Now()
		d, err := audio.Devices()
		if err != nil {
			m.listErr = "Can't list the inputs: " + err.Error()
		}
		if m.network {
			d = append(d, audio.Device{Name: audio.NetDevice})
		}
		m.devices = d
	}
	out := web.Mic{Live: live, Devices: m.devices, Active: m.active, Error: m.err}
	if out.Devices == nil {
		out.Devices = []audio.Device{}
	}
	if out.Error == "" {
		out.Error = m.listErr
	}
	if live && m.open && !paused {
		l := m.level
		out.Level = &l
	}
	return out
}

// listen keeps a microphone open: the one picked on the settings page, or
// the default. Picking another one there closes it and opens the new one.
// While listening is paused the microphone is closed. A microphone that
// cannot be opened, or stops sending audio, is tried again every few
// seconds, so plugging it in is enough.
// The network microphone (netAddr, with the panel token) is picked like
// any other input.
func listen(ctx context.Context, live *atomic.Pointer[config.Config], mic *micState, changed <-chan struct{}, segSamples int, segs chan<- []int16, netAddr, token string) {
	for {
		if live.Load().Paused {
			mic.set(func(m *micState) { m.active, m.open, m.err = "", false, "" })
			log.Print("microphone closed while paused")
			select {
			case <-changed:
				continue
			case <-ctx.Done():
				return
			}
		}
		name := live.Load().MicDevice
		mctx, cancel := context.WithCancel(ctx)
		go func() {
			select {
			case <-changed:
				cancel()
			case <-mctx.Done():
			}
		}()
		open := audio.Mic
		if name == audio.NetDevice && netAddr != "" {
			open = func(ctx context.Context, _ string, n int, _ time.Duration, out chan<- []int16, opened func(string), level func(float64)) error {
				return audio.NetMic(ctx, netAddr, token, n, out, opened, level)
			}
		}
		err := open(mctx, name, segSamples, time.Duration(live.Load().MicTimeoutSeconds)*time.Second, segs,
			func(using string) {
				log.Printf("microphone open: %s", cmp.Or(using, "the default input"))
				mic.set(func(m *micState) { m.active, m.open, m.err = using, true, "" })
			},
			func(dBFS float64) { mic.set(func(m *micState) { m.level = dBFS }) })
		cancel()
		mic.set(func(m *micState) { m.open = false })
		if err == nil {
			continue // another microphone was picked
		}
		log.Printf("microphone: %v; trying again in 5 s", err)
		mic.set(func(m *micState) { m.active, m.err = "", "Can't open the microphone: "+err.Error() })
		select {
		case <-time.After(5 * time.Second):
		case <-changed:
		case <-ctx.Done():
			return
		}
	}
}

// styleName names a picture style for the settings page: a preset's name,
// or "Your own" for a style typed in.
func styleName(style string) string { return imagegen.StyleName(style) }
