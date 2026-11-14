package topic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The external service gets the key and the window at its own /v1 address.
func TestExternal(t *testing.T) {
	var path, auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, auth = r.URL.Path, r.Header.Get("Authorization")
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{
			"content": `{"topic":"x","confidence":1,"visual_prompt":"y","sensitive":"none"}`}}}})
	}))
	defer srv.Close()
	env := map[string]string{"TOPIC_API_URL": srv.URL + "/api/v1/", "TOPIC_API_KEY": "k", "TOPIC_API_MODEL": "m"}
	e := ExternalFromEnv(func(k string) string { return env[k] })
	if !e.Ready() {
		t.Fatal("not ready with a key and a model")
	}
	if _, err := e.Extractor("", nil).Extract(context.Background(), "synthetic talk", nil); err != nil {
		t.Fatal(err)
	}
	if path != "/api/v1/chat/completions" || auth != "Bearer k" {
		t.Errorf("path %q auth %q", path, auth)
	}
	if n := ExternalFromEnv(func(string) string { return "" }); n.Ready() || n.Name() != " at api.openai.com" {
		t.Errorf("empty env: ready %v name %q", n.Ready(), n.Name())
	}
}
