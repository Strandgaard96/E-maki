package audio

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

// A sender with the right token gets its PCM cut into segments; one
// without is hung up on.
func TestNetMic(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	ln.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan []int16, 4)
	opened := make(chan string, 4)
	go NetMic(ctx, addr, "s3cret", 4, out, func(n string) { opened <- n }, nil)

	var c net.Conn
	for i := 0; i < 50; i++ {
		var err error
		if c, err = net.Dial("tcp", addr); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if c == nil {
		t.Fatal("NetMic did not listen")
	}
	pcm := make([]byte, 16)
	for i := range 8 {
		binary.LittleEndian.PutUint16(pcm[2*i:], uint16(int16(i-4)))
	}
	c.Write([]byte("s3cret\n"))
	c.Write(pcm[:5]) // half a sample: the next write completes it
	c.Write(pcm[5:])
	for _, want := range [][]int16{{-4, -3, -2, -1}, {0, 1, 2, 3}} {
		select {
		case got := <-out:
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("segment %v, want %v", got, want)
				}
			}
		case <-time.After(2 * time.Second):
			t.Fatal("no segment")
		}
	}
	if name := <-opened; name == "" {
		t.Error("opened was not told")
	}
	c.Close()

	bad, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	bad.Write([]byte("wrong\n"))
	bad.Write(pcm)
	select {
	case got := <-out:
		t.Fatalf("a sender without the token got through: %v", got)
	case <-time.After(300 * time.Millisecond):
	}
	bad.Close()
}
