// Package memory keeps the topics the room has talked about, with a weight
// that decays over time and grows each time a topic comes back. It decides
// which topics get an image and which are on the wall.
package memory

import (
	"math"
	"sort"
	"strings"
	"time"
)

// Decision says what Observe did with a topic.
type Decision string

const (
	NewTopic Decision = "new"      // first sighting that qualifies: make an image
	Bump     Decision = "bump"     // known topic: weight up, no new image
	Pending  Decision = "pending"  // seen, but not yet often enough to show
	LowConf  Decision = "low_conf" // below the confidence threshold
	Blocked  Decision = "blocked"  // removed by the household recently
	Capped   Decision = "capped"   // would be new, but today's image cap is spent
)

// Settings tune the memory. Diary mode uses MinMentions 2 ("seen twice"),
// party mode 1.
type Settings struct {
	HalfLife      time.Duration
	MinConfidence float64
	MinMentions   int
	DailyImageCap int
	MaxTiles      int
	MinWeight     float64 // below this a topic leaves the wall
	BlockFor      time.Duration
	// Newest orders the wall by when a topic went up, newest first, and
	// keeps tiles until newer ones push them off, faded or not ("rolling").
	// Otherwise the heaviest topic comes first and faded ones leave.
	Newest bool
	// Every is the least time between two new tiles going up; new topics
	// wait their turn in a queue. 0 puts each up at once.
	Every time.Duration
	// NewestBig puts the newest topic first (the big tile) without Newest.
	NewestBig bool
	// Quiet hours, in minutes after midnight local time: no new tiles go
	// up from QuietFrom until QuietTo. Equal values are off.
	QuietFrom, QuietTo int
	// Diary collects topics without making pictures; Edition picks the top
	// ones once a day at DiaryAt (minutes after midnight, local time).
	Diary   bool
	DiaryAt int
}

// quiet reports whether now falls in the quiet hours.
func (s Settings) quiet(now time.Time) bool {
	m := now.Hour()*60 + now.Minute()
	if s.QuietFrom == s.QuietTo {
		return false
	}
	if s.QuietFrom < s.QuietTo {
		return m >= s.QuietFrom && m < s.QuietTo
	}
	return m >= s.QuietFrom || m < s.QuietTo // over midnight
}

// Topic is one remembered topic.
type Topic struct {
	Label     string
	Prompt    string
	FirstSeen time.Time
	LastSeen  time.Time
	Mentions  int
	Pinned    bool
	HasImage  bool
	Shown     time.Time // when it went up on the wall; zero while it waits
	Made      time.Time // when it got its image
	queued    time.Time // its place in the queue: when it got its image, unless moved
	mentions  []mention
}

type mention struct {
	at   time.Time
	conf float64
}

// Weight is the sum over mentions of confidence * 0.5^(age/half-life).
func (t *Topic) Weight(now time.Time, halfLife time.Duration) float64 {
	var w float64
	for _, m := range t.mentions {
		w += m.conf * math.Pow(0.5, float64(now.Sub(m.at))/float64(halfLife))
	}
	return w
}

// Memory is not safe for concurrent use; the pipeline owns it.
type Memory struct {
	s       Settings
	topics  map[string]*Topic
	blocked map[string]time.Time
	images  map[string]int // images made per day, keyed "2006-01-02"
	shown   time.Time      // when the last new tile went up
	edition time.Time      // when diary mode last picked its topics
}

func New(s Settings) *Memory {
	return &Memory{s: s, topics: map[string]*Topic{}, blocked: map[string]time.Time{}, images: map[string]int{}}
}

// Key normalises a label so "Ski trip to Lofoten" and "ski trip to lofoten "
// are the same topic.
func Key(label string) string {
	return strings.Join(strings.Fields(strings.ToLower(label)), " ")
}

// Small LLMs often rephrase a topic ("online dating awkwardness", then
// "online dating disaster") instead of reusing the label, so labels that
// share at least two content words, covering half of the shorter label, are
// the same topic.
func similar(a, b string) bool {
	wa, wb := contentWords(a), contentWords(b)
	shared := 0
	for w := range wa {
		if wb[w] {
			shared++
		}
	}
	return shared >= 2 && 2*shared >= min(len(wa), len(wb))
}

var filler = map[string]bool{"the": true, "and": true, "for": true, "with": true, "about": true, "from": true, "into": true, "over": true}

func contentWords(label string) map[string]bool {
	ws := map[string]bool{}
	for _, w := range strings.Fields(Key(label)) {
		w = strings.Trim(w, ".,!?;:'\"()")
		if len(w) >= 3 && !filler[w] {
			ws[w] = true
		}
	}
	return ws
}

// Canonical returns the label of a remembered (or recently removed) topic
// that label is the same as, or label itself if it is a new topic.
func (m *Memory) Canonical(label string) string {
	return m.canonicalKey(label, Key(label))
}

func (m *Memory) canonicalKey(label, k string) string {
	if t, ok := m.topics[k]; ok {
		return t.Label
	}
	if _, ok := m.blocked[k]; ok {
		return label
	}
	for _, t := range m.topics {
		if similar(label, t.Label) {
			return t.Label
		}
	}
	for b := range m.blocked {
		if similar(label, b) {
			return b
		}
	}
	return label
}

// Observe records a topic sighting and returns what to do with it. A label
// similar to a known topic counts as that topic (see Canonical).
func (m *Memory) Observe(label, prompt string, conf float64, now time.Time) Decision {
	label = m.Canonical(label)
	k := Key(label)
	if until, ok := m.blocked[k]; ok && now.Before(until) {
		return Blocked
	}
	if conf < m.s.MinConfidence {
		return LowConf
	}
	t, ok := m.topics[k]
	if !ok {
		t = &Topic{Label: label, FirstSeen: now}
		m.topics[k] = t
	}
	t.Prompt = prompt
	t.LastSeen = now
	t.Mentions++
	t.mentions = append(t.mentions, mention{now, conf})
	switch {
	case t.HasImage:
		return Bump
	case m.s.Diary || t.Mentions < m.s.MinMentions:
		return Pending
	case m.images[now.Format("2006-01-02")] >= m.s.DailyImageCap:
		return Capped
	}
	m.made(t, now)
	return NewTopic
}

func (m *Memory) made(t *Topic, now time.Time) {
	m.images[now.Format("2006-01-02")]++
	t.HasImage = true
	t.Made, t.queued = now, now
}

// Edition is diary mode's daily pick: once DiaryAt has passed since the
// last pick, it returns the heaviest topics without a picture that were
// seen at least MinMentions times and have not faded, as many as the wall holds and today's
// image limit allows. Outside diary mode, or when it is not time, it
// returns nil. The first call only starts the clock, so a start or a
// switch to diary mode waits for the next DiaryAt.
func (m *Memory) Edition(now time.Time) []Topic {
	if !m.s.Diary {
		return nil
	}
	if m.edition.IsZero() {
		m.edition = now
		return nil
	}
	if due := m.lastDiaryAt(now); !m.edition.Before(due) {
		return nil
	}
	m.edition = now
	var out []Topic
	for _, t := range m.topics {
		// a topic that faded below MinWeight would leave the wall at once
		if !t.HasImage && t.Mentions >= m.s.MinMentions && t.Weight(now, m.s.HalfLife) >= m.s.MinWeight {
			out = append(out, *t)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Weight(now, m.s.HalfLife) > out[j].Weight(now, m.s.HalfLife)
	})
	n := min(m.s.MaxTiles, m.s.DailyImageCap-m.ImagesToday(now))
	if len(out) > n {
		out = out[:max(n, 0)]
	}
	return out
}

// lastDiaryAt is the most recent DiaryAt at or before now.
func (m *Memory) lastDiaryAt(now time.Time) time.Time {
	at := time.Date(now.Year(), now.Month(), now.Day(), m.s.DiaryAt/60, m.s.DiaryAt%60, 0, 0, now.Location())
	if at.After(now) {
		at = at.AddDate(0, 0, -1)
	}
	return at
}

// NextEdition says how many topics diary mode has collected for its next
// pick, and when that is. Outside diary mode next is zero.
func (m *Memory) NextEdition(now time.Time) (collected int, next time.Time) {
	if !m.s.Diary {
		return 0, time.Time{}
	}
	for _, t := range m.topics {
		if !t.HasImage && t.Mentions >= m.s.MinMentions {
			collected++
		}
	}
	return collected, m.lastDiaryAt(now).AddDate(0, 0, 1)
}

// Made gives a collected topic its picture, counted against today's limit,
// and puts it in the queue. It reports whether the topic is still there
// without a picture (it may have been removed meanwhile).
func (m *Memory) Made(label string, now time.Time) bool {
	t, ok := m.topics[Key(label)]
	if !ok || t.HasImage {
		return false
	}
	m.made(t, now)
	return true
}

// NoImage undoes Observe's NewTopic when the picture could not be made: the
// topic no longer counts as painted or against the day's cap, so its next
// mention tries again.
func (m *Memory) NoImage(label string) {
	t, ok := m.topics[Key(label)]
	if !ok || !t.HasImage {
		return
	}
	if day := t.Made.Format("2006-01-02"); m.images[day] > 0 {
		m.images[day]--
	}
	t.HasImage, t.Made, t.queued = false, time.Time{}, time.Time{}
}

// Reveal puts the longest-waiting new topic up on the wall if one waits and
// Every has passed since the last one; with Every 0 it puts all waiting
// topics up. It reports whether any went up.
func (m *Memory) Reveal(now time.Time) bool {
	any := false
	if m.s.quiet(now) {
		return false
	}
	for next := m.next(); next != nil; next = m.next() {
		if m.s.Every > 0 && !m.shown.IsZero() && now.Sub(m.shown) < m.s.Every {
			break
		}
		next.Shown, m.shown, any = now, now, true
		if m.s.Every > 0 {
			break
		}
	}
	return any
}

func (m *Memory) next() *Topic {
	var next *Topic
	for _, t := range m.topics {
		if t.HasImage && t.Shown.IsZero() && (next == nil || t.queued.Before(next.queued)) {
			next = t
		}
	}
	return next
}

// Waiting lists the topics that have a picture and wait to go up, the next
// one first.
func (m *Memory) Waiting() []*Topic {
	var out []*Topic
	for _, t := range m.topics {
		if t.HasImage && t.Shown.IsZero() {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].queued.Before(out[j].queued) })
	return out
}

// MoveUp lets a waiting topic swap places with the one before it. It
// reports whether the topic waits.
func (m *Memory) MoveUp(label string) bool {
	w := m.Waiting()
	for i, t := range w {
		if Key(t.Label) == Key(label) {
			if i > 0 {
				w[i].queued, w[i-1].queued = w[i-1].queued, w[i].queued
				if w[i].queued.Equal(w[i-1].queued) {
					w[i].queued = w[i-1].queued.Add(-time.Nanosecond)
				}
			}
			return true
		}
	}
	return false
}

// ShowNow puts a waiting topic up at once, whatever the queue or quiet
// hours say, and starts the wait for the next one from now. It reports
// whether the topic waited.
func (m *Memory) ShowNow(label string, now time.Time) bool {
	for _, t := range m.Waiting() {
		if Key(t.Label) == Key(label) {
			t.Shown, m.shown = now, now
			return true
		}
	}
	return false
}

// Queue says how many new topics wait to go up, and when the next may.
func (m *Memory) Queue(now time.Time) (waiting int, next time.Time) {
	for _, t := range m.topics {
		if t.HasImage && t.Shown.IsZero() {
			waiting++
		}
	}
	if waiting > 0 {
		next = now
		if at := m.shown.Add(m.s.Every); m.s.Every > 0 && !m.shown.IsZero() && at.After(now) {
			next = at
		}
		if m.s.quiet(next) {
			// the end of the quiet hours, today or tomorrow
			end := time.Date(next.Year(), next.Month(), next.Day(), m.s.QuietTo/60, m.s.QuietTo%60, 0, 0, next.Location())
			if !end.After(next) {
				end = end.AddDate(0, 0, 1)
			}
			next = end
		}
	}
	return waiting, next
}

// SetSettings changes the tuning, e.g. when the mode is switched. Topics
// already remembered keep their mentions; their weights follow the new
// half-life from now on.
func (m *Memory) SetSettings(s Settings) { m.s = s }

// Settings returns the settings in force.
func (m *Memory) Settings() Settings { return m.s }

// ImagesToday is how many images were made on now's day.
func (m *Memory) ImagesToday(now time.Time) int { return m.images[now.Format("2006-01-02")] }

// Remove takes a topic off the wall and keeps it away for BlockFor.
func (m *Memory) Remove(label string, now time.Time) {
	k := Key(label)
	delete(m.topics, k)
	m.blocked[k] = now.Add(m.s.BlockFor)
}

// Prompt is the picture description of a topic that has a picture, on the
// wall or waiting; ok is false for any other topic.
func (m *Memory) Prompt(label string) (prompt string, ok bool) {
	t, ok := m.topics[Key(label)]
	if !ok || !t.HasImage {
		return "", false
	}
	return t.Prompt, true
}

// Redrawn records a new picture for a topic that already had one: it
// counts against the day's cap and becomes the picture's made time. It
// reports whether the topic still has a picture.
func (m *Memory) Redrawn(label string, now time.Time) bool {
	t, ok := m.topics[Key(label)]
	if !ok || !t.HasImage {
		return false
	}
	m.images[now.Format("2006-01-02")]++
	t.Made = now
	return true
}

// Pin keeps a topic on the wall regardless of its weight.
func (m *Memory) Pin(label string, pinned bool) {
	if t, ok := m.topics[Key(label)]; ok {
		t.Pinned = pinned
	}
}

// Wall returns the topics to show: pinned topics first, then the topics
// that went up, heaviest (or with Newest, newest) first, up to MaxTiles.
// Without Newest, topics that faded below MinWeight leave.
func (m *Memory) Wall(now time.Time) []*Topic {
	var out []*Topic
	for _, t := range m.topics {
		if t.HasImage && !t.Shown.IsZero() && (t.Pinned || m.s.Newest || t.Weight(now, m.s.HalfLife) >= m.s.MinWeight) {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned
		}
		if m.s.Newest && !out[i].Shown.Equal(out[j].Shown) {
			return out[i].Shown.After(out[j].Shown)
		}
		wi, wj := out[i].Weight(now, m.s.HalfLife), out[j].Weight(now, m.s.HalfLife)
		if wi != wj {
			return wi > wj
		}
		return out[i].FirstSeen.Before(out[j].FirstSeen)
	})
	if m.s.NewestBig && !m.s.Newest {
		newest := 0
		for i, t := range out {
			if t.Shown.After(out[newest].Shown) {
				newest = i
			}
		}
		if len(out) > 0 {
			t := out[newest]
			copy(out[1:newest+1], out[:newest])
			out[0] = t
		}
	}
	if len(out) > m.s.MaxTiles {
		out = out[:m.s.MaxTiles]
	}
	return out
}

// Clear takes every tile that is not pinned off the wall and forgets it,
// without keeping it away like Remove does. It returns the forgotten keys.
func (m *Memory) Clear() []string {
	var gone []string
	for k, t := range m.topics {
		if !t.Pinned {
			gone = append(gone, k)
			delete(m.topics, k)
		}
	}
	return gone
}

// Prune forgets what no longer matters, so a frame that runs for months
// does not fill its memory: topics that faded below MinWeight, all but the
// heaviest 3*MaxTiles topics, blocks that ran out and image counts of past
// days. Pinned topics stay. It returns the keys of the forgotten topics.
func (m *Memory) Prune(now time.Time) []string {
	var keep []*Topic
	var gone []string
	onWall := map[*Topic]bool{}
	for _, t := range m.Wall(now) {
		onWall[t] = true
	}
	for k, t := range m.topics {
		if t.Pinned || onWall[t] || t.Weight(now, m.s.HalfLife) >= m.s.MinWeight {
			keep = append(keep, t)
		} else {
			gone = append(gone, k)
		}
	}
	if limit := 3 * m.s.MaxTiles; limit > 0 && len(keep) > limit {
		sort.Slice(keep, func(i, j int) bool {
			if a, b := keep[i].Pinned || onWall[keep[i]], keep[j].Pinned || onWall[keep[j]]; a != b {
				return a
			}
			return keep[i].Weight(now, m.s.HalfLife) > keep[j].Weight(now, m.s.HalfLife)
		})
		for _, t := range keep[limit:] {
			if !t.Pinned && !onWall[t] {
				gone = append(gone, Key(t.Label))
			}
		}
	}
	for _, k := range gone {
		delete(m.topics, k)
	}
	for k, until := range m.blocked {
		if !now.Before(until) {
			delete(m.blocked, k)
		}
	}
	for day := range m.images {
		if day != now.Format("2006-01-02") {
			delete(m.images, day)
		}
	}
	return gone
}

// Recent returns up to n labels, most recently seen first, to show the LLM.
func (m *Memory) Recent(n int) []string {
	var ts []*Topic
	for _, t := range m.topics {
		ts = append(ts, t)
	}
	sort.Slice(ts, func(i, j int) bool { return ts[i].LastSeen.After(ts[j].LastSeen) })
	var out []string
	for i := 0; i < len(ts) && i < n; i++ {
		out = append(out, ts[i].Label)
	}
	return out
}

// Kept is a topic with a picture, as saved to keep the frame across a
// restart: labels, the picture description and times, never what was said.
type Kept struct {
	Label     string    `json:"label"`
	Prompt    string    `json:"prompt"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
	Made      time.Time `json:"made"`
	Shown     time.Time `json:"shown"` // zero while it waits in the queue
	Queued    time.Time `json:"queued"`
	Mentions  int       `json:"mentions"`
	Weight    float64   `json:"weight"` // at LastSeen, so tiles keep their size and keep fading
	Pinned    bool      `json:"pinned"`
	// Collected is a diary topic that waits for the daily pick, with no
	// picture yet.
	Collected bool `json:"collected,omitempty"`
}

// Keep lists the topics that have a picture, on the wall or waiting, and
// in diary mode the topics collected for the next pick.
func (m *Memory) Keep() []Kept {
	var out []Kept
	for _, t := range m.topics {
		if t.HasImage || m.s.Diary {
			out = append(out, Kept{t.Label, t.Prompt, t.FirstSeen, t.LastSeen, t.Made, t.Shown, t.queued,
				t.Mentions, t.Weight(t.LastSeen, m.s.HalfLife), t.Pinned, !t.HasImage})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Queued.Before(out[j].Queued) })
	return out
}

// Restore brings kept topics back after a restart.
func (m *Memory) Restore(kept []Kept) {
	for _, k := range kept {
		t := &Topic{Label: k.Label, Prompt: k.Prompt, FirstSeen: k.FirstSeen, LastSeen: k.LastSeen, Made: k.Made,
			Shown: k.Shown, queued: k.Queued, Mentions: k.Mentions, Pinned: k.Pinned, HasImage: !k.Collected,
			mentions: []mention{{k.LastSeen, k.Weight}}}
		m.topics[Key(k.Label)] = t
		if t.Shown.After(m.shown) {
			m.shown = t.Shown
		}
	}
}

// Counts is what else a restart should not forget: how many pictures were
// made per day (the daily cap) and which removed topics stay away until
// when. Labels and numbers only.
type Counts struct {
	Images  map[string]int       `json:"images"`
	Blocked map[string]time.Time `json:"blocked"`
}

// Counts returns the image counts and blocks in force.
func (m *Memory) Counts() Counts { return Counts{m.images, m.blocked} }

// RestoreCounts brings them back after a restart.
func (m *Memory) RestoreCounts(c Counts) {
	for k, v := range c.Images {
		m.images[k] = v
	}
	for k, v := range c.Blocked {
		m.blocked[k] = v
	}
}
