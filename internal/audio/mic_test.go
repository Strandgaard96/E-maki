package audio

import "testing"

func TestOnlyMicrophonesAreListed(t *testing.T) {
	for name, want := range map[string]bool{
		"RDP Source":                     true,
		"USB PnP Sound Device Mono":      true,
		"Monitor of RDP Sink":            false,
		"Monitor of Built-in Audio":      false,
		"Discard all samples (playback)": false,
	} {
		if got := microphone(name); got != want {
			t.Errorf("microphone(%q) = %v, want %v", name, got, want)
		}
	}
}
