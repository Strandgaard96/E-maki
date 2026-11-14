package audio

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestWAVRoundTrip(t *testing.T) {
	pcm := []int16{0, 1, -1, 32767, -32768, 1234}
	path := filepath.Join(t.TempDir(), "a.wav")
	if err := os.WriteFile(path, EncodeWAV(pcm), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadWAV(path)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, pcm) {
		t.Fatalf("got %v, want %v", got, pcm)
	}
}

func TestLevelDBFS(t *testing.T) {
	silence := make([]int16, 16000)
	loud := make([]int16, 16000)
	for i := range loud {
		if i%2 == 0 {
			loud[i] = 16384
		} else {
			loud[i] = -16384
		}
	}
	if l := LevelDBFS(silence); l > -100 {
		t.Errorf("silence at %.1f dBFS", l)
	}
	if l := LevelDBFS(loud); l < -6.1 || l > -5.9 {
		t.Errorf("half-scale square wave at %.1f dBFS, want -6", l)
	}
}
