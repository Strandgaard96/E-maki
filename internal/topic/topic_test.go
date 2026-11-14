package topic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIExtract(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path %s", r.URL.Path)
		}
		var req struct {
			Model    string                     `json:"model"`
			Messages []struct{ Content string } `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "gemma3:4b" || !strings.Contains(req.Messages[1].Content, "Ski trip to Lofoten") {
			t.Errorf("unexpected request %+v", req)
		}
		content := `{"topic":"Ski trip to Lofoten","confidence":0.8,"visual_prompt":"snowy peaks over a fjord","sensitive":false}`
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
	}))
	defer srv.Close()
	r, err := OpenAI{URL: srv.URL, Model: "gemma3:4b"}.Extract(context.Background(), "vi skal til Lofoten", []string{"Ski trip to Lofoten"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Topic != "Ski trip to Lofoten" || r.Confidence != 0.8 {
		t.Fatalf("got %+v", r)
	}
}

func TestCheck(t *testing.T) {
	window := "skal vi ikke bare tage til Lofoten i februar, the snow there is amazing in february"
	ok := Result{Topic: "Ski trip to Lofoten", VisualPrompt: "snowy peaks over a fjord, flat colours"}
	if err := Check(ok, window, nil); err != nil {
		t.Fatalf("clean result rejected: %v", err)
	}
	for name, r := range map[string]Result{
		"sensitive": {Topic: "x", VisualPrompt: "y", Sensitive: "health"},
		"empty":     {Topic: " ", VisualPrompt: "y"},
		"quote":     {Topic: "snow", VisualPrompt: "a poster: the snow there is amazing"},
		"long":      {Topic: "x", VisualPrompt: strings.Repeat("word ", 41)},
		"number":    {Topic: "x", VisualPrompt: "a door with the code 4821 on it", Sensitive: "none"},
		"wordy":     {Topic: strings.Repeat("word ", 11), VisualPrompt: "y", Sensitive: "none"},
		// With a block list the gate fails closed on anything it does not know.
		"unknown": {Topic: "x", VisualPrompt: "y", Sensitive: "medical"},
		"missing": {Topic: "x", VisualPrompt: "y"},
		"bool":    {Topic: "x", VisualPrompt: "y", Sensitive: "unspecified"},
	} {
		if Check(r, window, []string{"health"}) == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// The sensitive gate is off unless the category is configured.
	if err := Check(Result{Topic: "x", VisualPrompt: "y", Sensitive: "health"}, window, []string{"money"}); err != nil {
		t.Errorf("health topic blocked with only money configured: %v", err)
	}
}

// Synthetic transcript: a translated prompt never quotes it, so names are
// matched on their own.
func TestCheckNames(t *testing.T) {
	window := "Jeg talte med Anna i går. Hun vil til Lofoten i februar\nAnd on Friday I saw the cat on the roof"
	for prompt, ok := range map[string]bool{
		"A woman skiing down a snowy mountain":     true,
		"Anna skiing down a snowy mountain":        false,
		"A skier waving at Anna's cabin":           false,
		"A skier in the Lofoten mountains":         false,
		"A cat on a Friday, and I am surprised":    true, // weekdays and "I" are not names
		"Jeg hun and other sentence-initial words": true,
	} {
		if err := CheckNames(Result{VisualPrompt: prompt}, window); (err == nil) != ok {
			t.Errorf("%q: %v, want ok=%v", prompt, err, ok)
		}
	}
}

func TestCheckNever(t *testing.T) {
	never := []string{"meeting", "Anna", "  ", "quarterly report"}
	for _, c := range []struct {
		topic, prompt string
		ok            bool
	}{
		{"Rollerblading date", "Two people rollerblading by the sea", true},
		{"Endless meetings", "A tired office worker", false},
		{"Sunday plans", "A girl named ANNA's kite on a hill", false},
		{"Office", "A man buried in a Quarterly-Report", false},
		{"Office", "A quarterly party with a report card", true},
		{"Meetingroom design", "A bright room", true},
		{"Annabelle the cat", "A cat", true},
	} {
		err := CheckNever(Result{Topic: c.topic, VisualPrompt: c.prompt}, never)
		if (err == nil) != c.ok {
			t.Errorf("%q / %q: %v, want ok=%v", c.topic, c.prompt, err, c.ok)
		}
	}
	if CheckNever(Result{Topic: "anything"}, nil) != nil {
		t.Error("an empty list refused a topic")
	}
}

// A configured prompt replaces the default, but the privacy rules and JSON
// format are always sent.
func TestOpenAIConfiguredPrompt(t *testing.T) {
	var req struct {
		Temperature float64                    `json:"temperature"`
		Reasoning   string                     `json:"reasoning_effort"`
		Messages    []struct{ Content string } `json:"messages"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&req)
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{
			"content": `{"topic":"x","confidence":1,"visual_prompt":"y","sensitive":"none"}`}}}})
	}))
	defer srv.Close()
	zero := 0.0
	o := OpenAI{URL: srv.URL, Model: "m", Prompt: "Name the topic in three words.", Temperature: &zero, Reasoning: "none"}
	if _, err := o.Extract(context.Background(), "synthetic talk", nil); err != nil {
		t.Fatal(err)
	}
	if user := req.Messages[1].Content; !strings.Contains(user, "<transcript>\nsynthetic talk\n</transcript>") {
		t.Errorf("transcript is not marked as data: %q", user)
	}
	sys := req.Messages[0].Content
	if !strings.HasPrefix(sys, "Name the topic in three words.") || strings.Contains(sys, "Too general") {
		t.Errorf("system prompt does not use the configured prompt: %q", sys)
	}
	if !strings.Contains(sys, "Never include names of people") || !strings.Contains(sys, "Answer only with JSON") {
		t.Errorf("privacy rules or JSON format missing: %q", sys)
	}
	if req.Temperature != 0 || req.Reasoning != "none" {
		t.Errorf("temperature %v reasoning %q", req.Temperature, req.Reasoning)
	}
}

// The answer's shape is asked for as a JSON schema; a server that refuses
// it is asked again in plain JSON mode, and a fenced answer still parses.
func TestOpenAISchemaFallback(t *testing.T) {
	var formats []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ResponseFormat struct{ Type string } `json:"response_format"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		formats = append(formats, req.ResponseFormat.Type)
		if req.ResponseFormat.Type == "json_schema" {
			http.Error(w, "no schemas here", http.StatusBadRequest)
			return
		}
		content := "```json\n{\"topic\":\"\",\"confidence\":0.1,\"visual_prompt\":\"\",\"sensitive\":\"none\"}\n```"
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
	}))
	defer srv.Close()
	r, err := OpenAI{URL: srv.URL, Model: "m"}.Extract(context.Background(), "hello there", nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(formats, ",") != "json_schema,json_object" {
		t.Errorf("formats %v", formats)
	}
	if err := Check(r, "hello there", []string{"health"}); err != ErrNoTopic {
		t.Errorf("empty answer: %v, want ErrNoTopic", err)
	}
}
