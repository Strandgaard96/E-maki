package audio

import "math"

// LevelDBFS is the RMS level of pcm in dB relative to full scale.
func LevelDBFS(pcm []int16) float64 {
	if len(pcm) == 0 {
		return math.Inf(-1)
	}
	var sum float64
	for _, s := range pcm {
		v := float64(s) / 32768
		sum += v * v
	}
	return 10 * math.Log10(sum/float64(len(pcm))+1e-20)
}

// LoudestDBFS is the level of the loudest 200 ms in pcm: the same chunks
// the settings page's meter shows. A few seconds of talk in an otherwise
// quiet segment count as speech, where the whole segment's average would
// sit far below.
func LoudestDBFS(pcm []int16) float64 {
	loudest := math.Inf(-1)
	for i := 0; i < len(pcm); i += SampleRate / 5 {
		loudest = max(loudest, LevelDBFS(pcm[i:min(i+SampleRate/5, len(pcm))]))
	}
	return loudest
}
