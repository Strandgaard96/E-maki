package topic

import (
	"context"
	"sort"
	"strings"
)

// Fake picks the most frequent longer word in the window. It lets the whole
// pipeline run offline in tests and on a laptop without an LLM.
type Fake struct{}

var stop = map[string]bool{"about": true, "think": true, "really": true, "should": true, "would": true, "there": true, "their": true, "which": true, "because": true}

func (Fake) Extract(_ context.Context, window string, _ []string) (Result, error) {
	count := map[string]int{}
	for _, w := range strings.Fields(strings.ToLower(window)) {
		w = strings.Trim(w, ".,!?;:\"'()")
		if len([]rune(w)) >= 5 && !stop[w] {
			count[w]++
		}
	}
	var words []string
	for w := range count {
		words = append(words, w)
	}
	if len(words) == 0 {
		return Result{}, nil
	}
	sort.Slice(words, func(i, j int) bool {
		if count[words[i]] != count[words[j]] {
			return count[words[i]] > count[words[j]]
		}
		return words[i] < words[j]
	})
	w := words[0]
	return Result{Topic: w, Confidence: 0.9, VisualPrompt: "a flat linocut poster of " + w, Sensitive: "none"}, nil
}
