// Package pipeline connects transcribed speech to the wall: a rolling
// transcript window (RAM only) feeds the topic LLM, the memory decides, new
// topics get an image, and the page is redrawn for the panel.
package pipeline

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Strandgaard96/e-maki/internal/collage"
	"github.com/Strandgaard96/e-maki/internal/imagegen"
	"github.com/Strandgaard96/e-maki/internal/memory"
	"github.com/Strandgaard96/e-maki/internal/panel"
	"github.com/Strandgaard96/e-maki/internal/timeline"
	"github.com/Strandgaard96/e-maki/internal/topic"
)

type Pipeline struct {
	Mem            *memory.Memory
	Extractor      topic.Extractor
	Images         imagegen.Provider
	Panel          *panel.Server
	WindowSegments int
	ExtractEvery   int
	BlockSensitive []string        // sensitive categories never sent to the image API
	BlockNames     bool            // refuse image prompts that repeat a name from the transcript
	NeverPaint     []string        // words and phrases that never become a picture
	HideCaptions   bool            // draw the pictures without their captions
	Style          string          // the picture style's name, kept with each picture
	FrameURL       string          // where the picture can be seen, for the log
	KeepDir        string          // where pictures are kept across a restart; "" keeps them in memory only (see keep.go)
	Timeline       *timeline.Store // where every new picture is saved; nil saves none
	Size           image.Point     // the page as the viewer sees it; zero is collage.DefaultSize
	Now            func() time.Time

	mu     sync.Mutex
	window []string // transcript text: never logged, stored or sent beyond the LLM
	fed    int
	images map[string]image.Image
	styles map[string]string // the style each picture was made in
	drawn  bool              // a page has been drawn
	shown  []string          // the tiles on that page, in order
	paused bool              // listening is paused: segments are dropped unheard

	start     sync.Once
	jobs      chan job       // pictures to make, see paint
	painting  sync.WaitGroup // pictures asked for and not done
	redrawing sync.Mutex     // one picture redrawn at a time

	activity Activity
}

// Activity counts what the pipeline did since it started, for the settings
// page. It holds levels and counts only, never what was said.
type Activity struct {
	Heard     int       `json:"heard"`      // segments with speech, transcribed
	Quiet     int       `json:"quiet"`      // segments too quiet to transcribe
	Topics    int       `json:"topics"`     // topics the LLM named
	HeldBack  int       `json:"held_back"`  // topics refused before any image request
	Dropped   int       `json:"dropped"`    // segments dropped because the topic model fell behind
	LastLevel float64   `json:"last_level"` // dBFS of the last segment
	LastLang  string    `json:"last_lang"`  // language of the last transcribed segment
	LastAt    time.Time `json:"last_at"`
}

func (p *Pipeline) count(f func(*Activity)) {
	p.mu.Lock()
	f(&p.activity)
	p.mu.Unlock()
}

// Heard records a segment's level, and its language if it was transcribed
// ("" when it was too quiet).
func (p *Pipeline) Heard(level float64, lang string, quiet bool) {
	p.count(func(a *Activity) {
		if quiet {
			a.Quiet++
		} else {
			a.Heard++
			a.LastLang = lang
		}
		a.LastLevel, a.LastAt = level, p.Now()
	})
}

// Dropped records a segment dropped unread because the topic model fell
// behind.
func (p *Pipeline) Dropped() { p.count(func(a *Activity) { a.Dropped++ }) }

// Activity returns the counts so far.
func (p *Pipeline) Activity() Activity {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.activity
}

// gate is what Feed needs from the settings the web page can change.
type gate struct {
	extractor      topic.Extractor
	images         imagegen.Provider
	blockSensitive []string
	blockNames     bool
	neverPaint     []string
	style          string
	timeline       *timeline.Store
}

// Feed adds one transcribed segment and, every ExtractEvery segments, asks
// for the topic. An empty text (a quiet segment) still takes a place in the
// window, so old talk ages out when the room goes quiet. Feed is called from
// one goroutine; the lock is not held during the LLM call, so a button
// press does not wait for it. Pictures are made beside it (see paint).
func (p *Pipeline) Feed(ctx context.Context, text string) error {
	window, heard, segments, recent, g := p.listen(text)
	if window == "" {
		return nil
	}
	start := time.Now()
	r, err := g.extractor.Extract(ctx, window, recent)
	if err != nil {
		return err
	}
	model := ""
	if s, ok := g.extractor.(fmt.Stringer); ok && s.String() != "" {
		model = " (" + s.String() + ")"
	}
	log.Printf("llm%s: %d of the last %d segments had speech, answered in %.1fs", model, heard, segments, time.Since(start).Seconds())
	err = topic.Check(r, window, g.blockSensitive)
	if err == nil && g.blockNames {
		err = topic.CheckNames(r, window)
	}
	if err == nil {
		err = topic.CheckNever(r, g.neverPaint)
	}
	if errors.Is(err, topic.ErrNoTopic) {
		log.Print("llm: no clear topic")
		return nil
	}
	if err != nil {
		// Log the reason only, never the label: a rejected topic is one that
		// was too private to send, so it does not belong in a log file either.
		log.Printf("topic rejected before any image request: %v", err)
		p.count(func(a *Activity) { a.HeldBack++ })
		return nil
	}
	now := p.Now()
	p.mu.Lock()
	label := p.Mem.Canonical(r.Topic)
	d := p.Mem.Observe(label, r.VisualPrompt, r.Confidence, now)
	p.activity.Topics++
	p.mu.Unlock()
	if label != r.Topic {
		log.Printf("topic %q is the same as %q", r.Topic, label)
	}
	flag := ""
	if c := string(r.Sensitive); c != "" && c != "none" {
		flag = ", flagged " + c + " but not blocked"
	}
	log.Printf("topic %q (confidence %.2f%s): %s", label, r.Confidence, flag, d)
	switch d {
	case memory.NewTopic:
		// The image prompt is the one thing allowed to leave the house, so it
		// may be logged; the transcript never is.
		log.Printf("image prompt: %q", r.VisualPrompt)
		p.mu.Lock()
		p.Mem.Painting(label)
		p.mu.Unlock()
		p.paint(job{label, r.VisualPrompt, g.images, g.style, g.timeline, now})
		return nil // the picture redraws the page when it is done
	case memory.Bump:
	case memory.Pending:
		// diary mode keeps what it collected across a restart, if asked
		p.mu.Lock()
		p.keep()
		p.mu.Unlock()
		return nil
	default:
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, k := range p.Mem.Prune(now) {
		p.forget(k)
	}
	// A mention that moves no tile leaves the page alone: an e-ink frame
	// takes a full refresh for every new page. Its caption catches up the
	// next time the page changes.
	if p.drawn && slices.Equal(p.order(now), p.shown) {
		p.keep()
		return nil
	}
	return p.redraw(now)
}

// order is the keys of the wall's tiles in page order. It needs p.mu held.
func (p *Pipeline) order(now time.Time) []string {
	var keys []string
	for _, t := range p.Mem.Wall(now) {
		keys = append(keys, memory.Key(t.Label))
	}
	return keys
}

// Wait blocks until the pictures asked for so far are made (or failed).
func (p *Pipeline) Wait() { p.painting.Wait() }

// job is a picture to make for a new topic.
type job struct {
	label, prompt string
	images        imagegen.Provider
	style         string
	timeline      *timeline.Store
	seen          time.Time
}

// paint hands a picture to the painter, so a slow image provider (a local
// model on a CPU takes minutes) does not hold up topic detection. Only when
// a few pictures already wait does it block until there is room.
func (p *Pipeline) paint(j job) {
	p.start.Do(func() {
		p.jobs = make(chan job, 4)
		go func() {
			for j := range p.jobs {
				p.draw(j)
				p.painting.Done()
			}
		}()
	})
	p.painting.Add(1)
	p.jobs <- j
}

// draw makes one picture and puts it in line for the frame.
func (p *Pipeline) draw(j job) {
	start := time.Now()
	img, err := j.images.Generate(context.Background(), j.prompt)
	if err != nil {
		// No picture: the topic tries again the next time it comes up,
		// and the failed call does not count against the daily cap.
		log.Printf("image: %v", err)
		p.mu.Lock()
		p.Mem.Painted(j.label)
		p.Mem.NoImage(j.label)
		p.mu.Unlock()
		return
	}
	log.Printf("image: done in %.1fs", time.Since(start).Seconds())
	toTimeline(j.timeline, j.label, j.prompt, j.style, j.seen, img)
	p.mu.Lock()
	defer p.mu.Unlock()
	img = collage.Fit(img, p.size())
	if !p.Mem.Painted(j.label) {
		return // removed or forgotten while it was being made
	}
	if p.images == nil {
		p.images, p.styles = map[string]image.Image{}, map[string]string{}
	}
	now := p.Now()
	p.images[memory.Key(j.label)] = img
	p.styles[memory.Key(j.label)] = j.style
	p.Mem.Reveal(now)
	for _, k := range p.Mem.Prune(now) {
		p.forget(k)
	}
	if err := p.redraw(now); err != nil {
		log.Printf("redraw: %v", err)
	}
}

// listen adds a segment to the window. When a topic call is due it returns
// the window's text, how many of its segments had speech, the labels
// already known and the current settings; otherwise window is "".
func (p *Pipeline) listen(text string) (window string, heard, segments int, recent []string, g gate) {
	p.mu.Lock()
	defer p.mu.Unlock()
	g = gate{p.Extractor, p.Images, p.BlockSensitive, p.BlockNames, p.NeverPaint, p.Style, p.Timeline}
	if p.paused {
		return
	}
	text = strings.TrimSpace(text)
	p.window = append(p.window, text)
	if len(p.window) > p.WindowSegments {
		p.window = p.window[len(p.window)-p.WindowSegments:]
	}
	if text == "" {
		return
	}
	p.fed++
	if p.fed%p.ExtractEvery != 0 {
		return
	}
	var spoken []string
	for _, t := range p.window {
		if t != "" {
			spoken = append(spoken, t)
		}
	}
	return strings.Join(spoken, "\n"), len(spoken), len(p.window), p.Mem.Recent(collage.MaxTiles), g
}

// Press does what a frame button is set to do (config.Settings.Button):
// "remove" takes the picture that went up last off the frame, "pin" pins it
// (or lets it fade again) and "next" puts the next waiting picture up now.
// "pause" changes a setting, so the settings page handles it.
func (p *Pipeline) Press(action string) {
	p.mu.Lock()
	var newest, next string
	var last time.Time
	pinned := false
	for _, t := range p.Mem.Wall(p.Now()) {
		if t.Shown.After(last) {
			newest, last, pinned = t.Label, t.Shown, t.Pinned
		}
	}
	if w := p.Mem.Waiting(); len(w) > 0 {
		next = w[0].Label
	}
	p.mu.Unlock()
	switch {
	case action == "remove" && newest != "":
		p.Remove(newest)
	case action == "pin" && newest != "" && p.Pin(newest, !pinned):
		log.Printf("button: pinned %v", !pinned)
	case action == "next" && next != "":
		p.Line("now", next)
	default:
		log.Printf("button %q: nothing to do", action)
	}
}

// Tile is a topic on the wall, as the web page lists it.
type Tile struct {
	Label    string    `json:"label"`
	Mentions int       `json:"mentions"`
	LastSeen time.Time `json:"last_seen"`
	Pinned   bool      `json:"pinned"`
	// Where the picture came from: the description sent to the image API
	// (never what was said), when it was made and in which style.
	Prompt string    `json:"prompt"`
	Made   time.Time `json:"made"`
	Style  string    `json:"style"`
	// Rect is where the tile sits on the page: x, y, width and height as
	// fractions of the page, so the settings page can point at it.
	Rect [4]float64 `json:"rect"`
}

// Wall lists the tiles on the page, in page order.
func (p *Pipeline) Wall() []Tile {
	p.mu.Lock()
	defer p.mu.Unlock()
	tiles := []Tile{}
	wall := p.Mem.Wall(p.Now())
	size := p.size()
	rects := collage.Rects(len(wall), size)
	w, h := float64(size.X), float64(size.Y)
	for i, t := range wall {
		tile := p.tile(t)
		if i < len(rects) {
			r := rects[i]
			tile.Rect = [4]float64{float64(r.Min.X) / w, float64(r.Min.Y) / h, float64(r.Dx()) / w, float64(r.Dy()) / h}
		}
		tiles = append(tiles, tile)
	}
	return tiles
}

func (p *Pipeline) tile(t *memory.Topic) Tile {
	return Tile{Label: t.Label, Mentions: t.Mentions, LastSeen: t.LastSeen, Pinned: t.Pinned,
		Prompt: t.Prompt, Made: t.Made, Style: p.styles[memory.Key(t.Label)]}
}

// Waiting lists the pictures waiting to go up, the next one first.
func (p *Pipeline) Waiting() []Tile {
	p.mu.Lock()
	defer p.mu.Unlock()
	tiles := []Tile{}
	for _, t := range p.Mem.Waiting() {
		tiles = append(tiles, p.tile(t))
	}
	return tiles
}

// Picture is a topic's picture as it was made, on the wall or waiting;
// nil if there is none.
func (p *Pipeline) Picture(label string) image.Image {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.images[memory.Key(label)]
}

// Line changes a waiting picture's place: "up" moves it one place nearer
// the frame, "now" puts it up at once and "drop" throws it away and keeps
// the topic off the frame for a while. It reports whether it was waiting.
func (p *Pipeline) Line(action, label string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.Now()
	switch action {
	case "up":
		ok := p.Mem.MoveUp(label)
		p.keep()
		return ok
	case "now":
		if !p.Mem.ShowNow(label, now) {
			return false
		}
		log.Printf("put %q up by hand", label)
	case "drop":
		waits := false
		for _, t := range p.Mem.Waiting() {
			waits = waits || memory.Key(t.Label) == memory.Key(label)
		}
		if !waits {
			return false
		}
		p.Mem.Remove(label, now)
		p.forget(memory.Key(label))
		log.Printf("dropped %q from the queue", label)
		p.keep()
		return true
	default:
		return false
	}
	if err := p.redraw(now); err != nil {
		log.Printf("redraw: %v", err)
	}
	return true
}

// ErrBusy is Redraw's answer while another picture is being redrawn.
var ErrBusy = errors.New("another picture is being redrawn")

// Redraw makes a new picture for a topic on the wall or in the queue, from
// its stored description in the current style. It counts against the
// day's cap, and only once the picture exists.
func (p *Pipeline) Redraw(ctx context.Context, label string) error {
	if !p.redrawing.TryLock() {
		return ErrBusy
	}
	defer p.redrawing.Unlock()
	p.mu.Lock()
	now := p.Now()
	prompt, ok := p.Mem.Prompt(label)
	images, style := p.Images, p.Style
	limit := p.Mem.Settings().DailyImageCap
	used := p.Mem.ImagesToday(now)
	p.mu.Unlock()
	switch {
	case !ok:
		return ErrGone
	case prompt == "":
		return errors.New("this picture has no description to redraw from")
	case used >= limit:
		return fmt.Errorf("today's limit of %d new pictures is used up", limit)
	}
	log.Printf("redraw %q, image prompt: %q", label, prompt)
	start := time.Now()
	img, err := images.Generate(ctx, prompt)
	if err != nil {
		return err
	}
	log.Printf("image: done in %.1fs", time.Since(start).Seconds())
	p.mu.Lock()
	defer p.mu.Unlock()
	now = p.Now()
	if !p.Mem.Redrawn(label, now) {
		return ErrGone // removed while it was drawn
	}
	toTimeline(p.Timeline, label, prompt, style, now, img)
	k := memory.Key(label)
	img = collage.Fit(img, p.size())
	p.images[k], p.styles[k] = img, style
	if !p.onWall(label) {
		p.keep()
		return nil
	}
	return p.redraw(now)
}

// ErrGone says a topic is no longer on the wall or in the queue.
var ErrGone = errors.New("that picture is no longer on the frame or in the queue")

// toTimeline adds a new picture to the timeline, if there is one.
func toTimeline(tl *timeline.Store, label, prompt, style string, now time.Time, img image.Image) {
	if tl == nil {
		return
	}
	if _, err := tl.Add(label, prompt, style, now, img); err != nil {
		log.Printf("timeline: %v", err)
	}
}

func (p *Pipeline) forget(k string) {
	delete(p.images, k)
	delete(p.styles, k)
}

// Tick puts the next waiting topic up when its turn has come. Call it
// every minute or so.
func (p *Pipeline) Tick() {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.Now()
	if !p.Mem.Reveal(now) {
		return
	}
	if err := p.redraw(now); err != nil {
		log.Printf("redraw: %v", err)
	}
}

// Edition is diary mode's daily painting: when the time has come, the top
// topics collected since the last one get their pictures, and they all go
// up together (quiet hours permitting), so the frame changes once a day.
// Call it every minute or so. The image calls are made without the lock.
func (p *Pipeline) Edition(ctx context.Context) {
	p.mu.Lock()
	picks := p.Mem.Edition(p.Now())
	images, style, tl := p.Images, p.Style, p.Timeline
	p.mu.Unlock()
	if len(picks) == 0 {
		return
	}
	log.Printf("diary: painting the top %d topics", len(picks))
	made := 0
	for _, t := range picks {
		log.Printf("image prompt: %q", t.Prompt)
		start := time.Now()
		img, err := images.Generate(ctx, t.Prompt)
		if err != nil {
			// the topic keeps its place and can be picked tomorrow
			log.Printf("image for %q: %v", t.Label, err)
			continue
		}
		log.Printf("image: done in %.1fs", time.Since(start).Seconds())
		p.mu.Lock()
		if p.Mem.Made(t.Label, p.Now()) {
			toTimeline(tl, t.Label, t.Prompt, style, p.Now(), img)
			img = collage.Fit(img, p.size())
			if p.images == nil {
				p.images, p.styles = map[string]image.Image{}, map[string]string{}
			}
			p.images[memory.Key(t.Label)] = img
			p.styles[memory.Key(t.Label)] = style
			made++
		}
		p.mu.Unlock()
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.Now()
	p.Mem.Reveal(now)
	for _, k := range p.Mem.Prune(now) {
		p.forget(k)
	}
	if made == 0 {
		p.keep()
		return
	}
	if err := p.redraw(now); err != nil {
		log.Printf("redraw: %v", err)
	}
}

// NextEdition says how many topics diary mode has collected and when it
// paints next; next is zero outside diary mode.
func (p *Pipeline) NextEdition() (collected int, next time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.Mem.NextEdition(p.Now())
}

// Queue says how many new topics wait to go up, and when the next may.
func (p *Pipeline) Queue() (waiting int, next time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.Mem.Queue(p.Now())
}

// ImagesToday is how many images were made today.
func (p *Pipeline) ImagesToday() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.Mem.ImagesToday(p.Now())
}

// Remove takes a tile off the wall and keeps the topic away for a while.
// It reports whether the topic was on the wall.
func (p *Pipeline) Remove(label string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.onWall(label) {
		return false
	}
	now := p.Now()
	p.Mem.Remove(label, now)
	p.forget(memory.Key(label))
	log.Printf("removed %q", label)
	if err := p.redraw(now); err != nil {
		log.Printf("redraw: %v", err)
	}
	return true
}

// Pin keeps a tile on the wall however long ago it was talked about, or
// lets it fade again. It reports whether the topic was on the wall.
func (p *Pipeline) Pin(label string, pinned bool) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.onWall(label) {
		return false
	}
	p.Mem.Pin(label, pinned)
	if err := p.redraw(p.Now()); err != nil {
		log.Printf("redraw: %v", err)
	}
	return true
}

func (p *Pipeline) onWall(label string) bool {
	for _, t := range p.Mem.Wall(p.Now()) {
		if memory.Key(t.Label) == memory.Key(label) {
			return true
		}
	}
	return false
}

// Options are the pipeline settings the settings page changes.
type Options struct {
	Extractor      topic.Extractor
	WindowSegments int
	ExtractEvery   int
	Images         imagegen.Provider
	BlockSensitive []string
	BlockNames     bool
	NeverPaint     []string
	HideCaptions   bool
	Style          string
	KeepDir        string
	Timeline       *timeline.Store
	Paused         bool
	Size           image.Point
}

// Configure changes the settings the settings page controls while the
// pipeline runs, and redraws the page if there is one, since the layout can
// change which tiles are on it. Pausing also empties the transcript window.
func (p *Pipeline) Configure(o Options, mem memory.Settings) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Images, p.BlockSensitive, p.BlockNames, p.HideCaptions = o.Images, o.BlockSensitive, o.BlockNames, o.HideCaptions
	p.Style, p.Timeline, p.NeverPaint, p.Size = o.Style, o.Timeline, o.NeverPaint, o.Size
	if p.KeepDir != o.KeepDir {
		if o.KeepDir == "" {
			if err := Forget(p.KeepDir); err != nil {
				log.Printf("keep pictures: %v", err)
			} else {
				log.Print("kept pictures deleted")
			}
		}
		p.KeepDir = o.KeepDir
		p.keep()
	}
	p.Extractor, p.WindowSegments, p.ExtractEvery = o.Extractor, o.WindowSegments, o.ExtractEvery
	if o.Paused && !p.paused {
		p.window = nil
		log.Print("listening paused")
	} else if !o.Paused && p.paused {
		log.Print("listening again")
	}
	p.paused = o.Paused
	p.Mem.SetSettings(mem)
	p.Mem.Reveal(p.Now())
	if !p.drawn {
		return nil
	}
	return p.redraw(p.Now())
}

// size needs p.mu held.
func (p *Pipeline) size() image.Point {
	if p.Size == (image.Point{}) {
		return collage.DefaultSize
	}
	return p.Size
}

// Paused reports whether listening is paused, so the microphone loop can
// drop segments before they are transcribed.
func (p *Pipeline) Paused() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.paused
}

// Clear takes every tile that is not pinned off the frame, to start fresh.
func (p *Pipeline) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, k := range p.Mem.Clear() {
		p.forget(k)
	}
	log.Print("frame cleared")
	if err := p.redraw(p.Now()); err != nil {
		log.Printf("redraw: %v", err)
	}
}

func (p *Pipeline) redraw(now time.Time) error {
	tiles := p.tiles(now, p.HideCaptions)
	log.Printf("wall: %d tiles, see %s", len(tiles), p.FrameURL)
	p.drawn, p.shown = true, p.order(now)
	p.keep()
	page := collage.Render(tiles, p.size())
	if p.paused {
		collage.Paused(page, len(tiles) == 0)
	}
	return p.Panel.SetPage(page)
}

// tiles are the wall's tiles as the page draws them. It needs p.mu held.
func (p *Pipeline) tiles(now time.Time, hideCaptions bool) []collage.Tile {
	var tiles []collage.Tile
	for _, t := range p.Mem.Wall(now) {
		tile := collage.Tile{Image: p.images[memory.Key(t.Label)]}
		if !hideCaptions {
			tile.Title = t.Label
			tile.Meta = strings.ToUpper(fmt.Sprintf("%s · ×%d", t.LastSeen.Format("Mon 15:04"), t.Mentions))
		}
		tiles = append(tiles, tile)
	}
	return tiles
}

// Draft draws the page as it would look with other layout settings, from
// the pictures already in memory. Nothing changes: the frame keeps its page.
// With no pictures yet it fills the layout with sample tiles, so a layout
// can be seen before the room has talked about anything.
func (p *Pipeline) Draft(mem memory.Settings, hideCaptions bool, size image.Point) *image.RGBA {
	p.mu.Lock()
	cur := p.Mem.Settings()
	p.Mem.SetSettings(mem)
	tiles := p.tiles(p.Now(), hideCaptions)
	p.Mem.SetSettings(cur)
	p.mu.Unlock()
	// Slots the pictures do not fill yet get sample tiles, so the layout shows.
	n := max(min(mem.MaxTiles, collage.MaxTiles), 1)
	if len(tiles) < n {
		tiles = append(tiles, samples(len(tiles), n, hideCaptions)...)
	}
	return collage.Render(tiles, size)
}

// samples are stand-in tiles for a layout preview, slots from up to n: flat
// shapes, no image API.
func samples(from, n int, hideCaptions bool) []collage.Tile {
	var out []collage.Tile
	for i := from; i < n; i++ {
		img, _ := imagegen.Placeholder{}.Generate(context.Background(), fmt.Sprintf("sample %d", i+3))
		t := collage.Tile{Image: img}
		if !hideCaptions {
			t.Title, t.Meta = fmt.Sprintf("Sample topic %d", i+1), "EXAMPLE"
		}
		out = append(out, t)
	}
	return out
}

// Start draws the first page, so the frame shows the empty page rather than
// nothing before the first topic.
func (p *Pipeline) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.drawn {
		return nil
	}
	return p.redraw(p.Now())
}
