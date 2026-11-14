// Package audio reads speech audio. Only WAV files for now; the live mic and
// ESP32 stream sources come with the hardware decision.
package audio

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

// SampleRate is what every Source delivers and what Whisper expects.
const SampleRate = 16000

// ReadWAV reads a 16 kHz mono 16-bit PCM WAV file into memory.
func ReadWAV(path string) ([]int16, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var hdr [12]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil || string(hdr[0:4]) != "RIFF" || string(hdr[8:12]) != "WAVE" {
		return nil, errors.New("not a WAV file")
	}
	for {
		var ch [8]byte
		if _, err := io.ReadFull(f, ch[:]); err != nil {
			return nil, errors.New("WAV has no data chunk")
		}
		size := binary.LittleEndian.Uint32(ch[4:])
		switch string(ch[0:4]) {
		case "fmt ":
			if size < 16 || size > 1024 {
				return nil, errors.New("WAV has a bad fmt chunk")
			}
			b := make([]byte, size)
			if _, err := io.ReadFull(f, b); err != nil {
				return nil, err
			}
			format, chans := binary.LittleEndian.Uint16(b[0:]), binary.LittleEndian.Uint16(b[2:])
			rate, bits := binary.LittleEndian.Uint32(b[4:]), binary.LittleEndian.Uint16(b[14:])
			if format != 1 || chans != 1 || rate != SampleRate || bits != 16 {
				return nil, fmt.Errorf("need 16 kHz mono 16-bit PCM, got format %d, %d ch, %d Hz, %d bit", format, chans, rate, bits)
			}
		case "data":
			pcm := make([]int16, size/2)
			return pcm, binary.Read(io.LimitReader(f, int64(size)), binary.LittleEndian, pcm)
		default:
			if _, err := f.Seek(int64(size+size%2), io.SeekCurrent); err != nil {
				return nil, err
			}
		}
	}
}

// EncodeWAV wraps PCM samples in an in-memory WAV, for sending to a
// speech-to-text server. Nothing is written to disk.
func EncodeWAV(pcm []int16) []byte {
	data := len(pcm) * 2
	b := make([]byte, 44+data)
	copy(b[0:], "RIFF")
	binary.LittleEndian.PutUint32(b[4:], uint32(36+data))
	copy(b[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(b[16:], 16)
	binary.LittleEndian.PutUint16(b[20:], 1)
	binary.LittleEndian.PutUint16(b[22:], 1)
	binary.LittleEndian.PutUint32(b[24:], SampleRate)
	binary.LittleEndian.PutUint32(b[28:], SampleRate*2)
	binary.LittleEndian.PutUint16(b[32:], 2)
	binary.LittleEndian.PutUint16(b[34:], 16)
	copy(b[36:], "data")
	binary.LittleEndian.PutUint32(b[40:], uint32(data))
	for i, s := range pcm {
		binary.LittleEndian.PutUint16(b[44+2*i:], uint16(s))
	}
	return b
}
