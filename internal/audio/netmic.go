package audio

import (
	"bufio"
	"context"
	"crypto/subtle"
	"errors"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

// NetDevice is the network microphone's name in the list of inputs.
const NetDevice = "Network microphone"

// NetMic listens on addr (e.g. ":10700") for a microphone elsewhere on the
// network: a TCP connection that streams raw 16 kHz mono 16-bit
// little-endian PCM, the format of
//
//	arecord -q -f S16_LE -r 16000 -c 1 -t raw | nc brain 10700
//
// or the ESP32 sketch in examples/esp32-mic. With a token, the sender first
// sends the token and a newline; anything else is hung up on. One sender at
// a time: a new connection replaces the old one, so a sender that rebooted
// is back at once. Segments, opened and level work as for Mic. The audio
// stays in memory. NetMic blocks until ctx ends.
func NetMic(ctx context.Context, addr, token string, segSamples int, out chan<- []int16, opened func(name string), level func(dBFS float64)) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	go func() { <-ctx.Done(); ln.Close() }()
	log.Printf("network microphone: waiting for a sender on %s", addr)
	var current net.Conn
	for {
		c, err := ln.Accept()
		if err != nil {
			if current != nil {
				current.Close()
			}
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if current != nil {
			current.Close() // the newest sender wins
		}
		current = c
		go receive(c, token, segSamples, out, opened, level)
	}
}

// receive reads one sender until it hangs up or is replaced.
func receive(c net.Conn, token string, segSamples int, out chan<- []int16, opened func(string), level func(float64)) {
	defer c.Close()
	from := c.RemoteAddr().String()
	r := bufio.NewReader(c)
	if token != "" {
		c.SetReadDeadline(time.Now().Add(5 * time.Second))
		line, err := r.ReadString('\n')
		if err != nil || subtle.ConstantTimeCompare([]byte(strings.TrimSpace(line)), []byte(token)) != 1 {
			log.Printf("network microphone: %s did not send the token, hung up", from)
			return
		}
		c.SetReadDeadline(time.Time{})
	}
	log.Printf("network microphone: %s connected", from)
	if opened != nil {
		opened(NetDevice)
	}
	seg := newSegmenter(segSamples, out, level)
	buf := make([]byte, 4096)
	for {
		// A sender that goes quiet without hanging up (Wi-Fi gone) is let go,
		// so a new connection is not needed to notice.
		c.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, err := r.Read(buf)
		seg.write(buf[:n])
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				log.Printf("network microphone: %s: %v", from, err)
			}
			log.Printf("network microphone: %s gone", from)
			return
		}
	}
}
