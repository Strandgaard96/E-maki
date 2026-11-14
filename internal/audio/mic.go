package audio

import (
	"context"
	"encoding/binary"
	"log"
	"strings"
	"sync"

	"github.com/gen2brain/malgo"
)

// Device is an audio input the machine offers.
type Device struct {
	Name    string `json:"name"`
	Default bool   `json:"default"` // the one the system picks when none is chosen

	id malgo.DeviceID
}

// Devices lists the inputs E-kiru could listen on.
func Devices() ([]Device, error) {
	mctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, err
	}
	defer func() { mctx.Uninit(); mctx.Free() }()
	return devices(mctx)
}

func devices(mctx *malgo.AllocatedContext) ([]Device, error) {
	infos, err := mctx.Devices(malgo.Capture)
	if err != nil {
		return nil, err
	}
	out := []Device{}
	for _, d := range infos {
		if !microphone(d.Name()) {
			continue
		}
		out = append(out, Device{Name: d.Name(), Default: d.IsDefault != 0, id: d.ID})
	}
	return out, nil
}

// microphone tells real inputs from the capture devices that are not:
// miniaudio's null backend (silence), and PulseAudio/PipeWire monitors,
// which loop back what the speakers play (on WSL, "Monitor of RDP Sink").
func microphone(name string) bool {
	return !strings.HasPrefix(name, "Discard all samples") && !strings.HasPrefix(name, "Monitor of ")
}

// Mic captures an input device at 16 kHz mono and sends one segment of
// segSamples samples at a time on out. name picks the device; "" or a name
// that is not there (unplugged) uses the system default. opened is told the
// name of the device that is listening, and level its loudness in dBFS a
// few times a second; either may be nil. Audio stays in memory. If the
// pipeline falls behind, segments are dropped rather than queued: a missed
// sentence does not matter for topic detection. Mic blocks until ctx ends.
func Mic(ctx context.Context, name string, segSamples int, out chan<- []int16, opened func(name string), level func(dBFS float64)) error {
	mctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return err
	}
	defer func() { mctx.Uninit(); mctx.Free() }()

	cfg := malgo.DefaultDeviceConfig(malgo.Capture)
	cfg.Capture.Format = malgo.FormatS16
	cfg.Capture.Channels = 1
	cfg.SampleRate = SampleRate

	// The list is only needed to find a chosen device or name the default,
	// so a backend that cannot list its devices still listens.
	using := ""
	list, _ := devices(mctx)
	for i := range list {
		if list[i].Name == name && name != "" {
			using = name
			cfg.Capture.DeviceID = list[i].id.Pointer()
			break
		}
		if list[i].Default && using == "" {
			using = list[i].Name
		}
	}
	if name != "" && using != name {
		log.Printf("microphone %q is not there: using the default instead", name)
	}

	var mu sync.Mutex
	seg := newSegmenter(segSamples, out, level)
	onData := func(_, in []byte, _ uint32) {
		mu.Lock()
		defer mu.Unlock()
		seg.write(in)
	}
	dev, err := malgo.InitDevice(mctx.Context, cfg, malgo.DeviceCallbacks{Data: onData})
	if err != nil {
		return err
	}
	defer dev.Uninit()
	if err := dev.Start(); err != nil {
		return err
	}
	if opened != nil {
		opened(using)
	}
	<-ctx.Done()
	return nil
}

// segmenter cuts 16-bit little-endian PCM into segments of n samples for
// out, and reports the loudness a few times a second to level (may be nil).
// A segment the pipeline is too busy for is dropped, not queued.
type segmenter struct {
	out   chan<- []int16
	level func(dBFS float64)
	n     int
	buf   []int16
	chunk []int16
	odd   []byte // half a sample left over from the last write
}

func newSegmenter(n int, out chan<- []int16, level func(float64)) *segmenter {
	return &segmenter{out: out, level: level, n: n, buf: make([]int16, 0, n), chunk: make([]int16, 0, SampleRate/5)}
}

func (g *segmenter) write(in []byte) {
	if len(g.odd) > 0 {
		in = append(g.odd, in...)
		g.odd = nil
	}
	for i := 0; i+1 < len(in); i += 2 {
		s := int16(binary.LittleEndian.Uint16(in[i:]))
		if g.level != nil {
			if g.chunk = append(g.chunk, s); len(g.chunk) == cap(g.chunk) {
				g.level(LevelDBFS(g.chunk))
				g.chunk = g.chunk[:0]
			}
		}
		g.buf = append(g.buf, s)
		if len(g.buf) == g.n {
			select {
			case g.out <- g.buf:
			default:
				log.Print("pipeline busy: dropped a segment")
			}
			g.buf = make([]int16, 0, g.n)
		}
	}
	if len(in)%2 == 1 {
		g.odd = []byte{in[len(in)-1]}
	}
}
