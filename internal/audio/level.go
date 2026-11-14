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
