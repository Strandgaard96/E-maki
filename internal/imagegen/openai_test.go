package imagegen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestOpenAI(t *testing.T) {
	pic := pngBytes(t, 48, 32)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" || r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("unexpected request %s auth %q", r.URL.Path, r.Header.Get("Authorization"))
		}
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		if req["model"] != "gpt-image-2" || req["size"] != "1536x1024" || !strings.HasPrefix(req["prompt"].(string), "a llama") ||
			!strings.Contains(req["prompt"].(string), DefaultStyle) {
			t.Errorf("request %v", req)
		}
		json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"b64_json": base64.StdEncoding.EncodeToString(pic)}}})
	}))
	defer srv.Close()
	img, err := OpenAI{BaseURL: srv.URL + "/v1", APIKey: "k", Model: "gpt-image-2", Size: "1536x1024"}.Generate(context.Background(), "a llama")
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 48 {
		t.Fatalf("got %v", img.Bounds())
	}
}

// A local sd-server: no key, no model, a 503 while it loads, then the image
// as a link-free b64_json. Errors there are a plain string.
func TestOpenAILocalRetriesWhileLoading(t *testing.T) {
	pic := pngBytes(t, 16, 12)
	retryWait = 0
	defer func() { retryWait = 5 * time.Second }()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		if _, ok := req["model"]; ok || r.Header.Get("Authorization") != "" {
			t.Errorf("local request sent a model or a key: %v", req)
		}
		if calls == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":"model is loading","code":"model_loading"}`))
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"b64_json": base64.StdEncoding.EncodeToString(pic)}}})
	}))
	defer srv.Close()
	img, err := OpenAI{BaseURL: srv.URL + "/v1"}.Generate(context.Background(), "a kite")
	if err != nil || calls != 2 || img.Bounds().Dx() != 16 {
		t.Fatalf("calls %d, err %v", calls, err)
	}
}

func TestOpenAIErrorsAndLinks(t *testing.T) {
	pic := pngBytes(t, 8, 8)
	mux := http.NewServeMux()
	mux.HandleFunc("/bad/images/generations", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"Your request was rejected by the safety system."}}`))
	})
	mux.HandleFunc("/link/images/generations", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"url": "http://" + r.Host + "/pic.png"}}})
	})
	mux.HandleFunc("/pic.png", func(w http.ResponseWriter, r *http.Request) { w.Write(pic) })
	srv := httptest.NewServer(mux)
	defer srv.Close()
	if _, err := (OpenAI{BaseURL: srv.URL + "/bad"}).Generate(context.Background(), "x"); err == nil || !strings.Contains(err.Error(), "safety system") {
		t.Errorf("err = %v, want the service's message", err)
	}
	img, err := OpenAI{BaseURL: srv.URL + "/link"}.Generate(context.Background(), "x")
	if err != nil || img.Bounds().Dx() != 8 {
		t.Fatalf("link: %v", err)
	}
}

func TestOpenRouter(t *testing.T) {
	pic := pngBytes(t, 40, 30)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model      string
			Modalities []string
			Messages   []struct{ Content string }
		}
		json.NewDecoder(r.Body).Decode(&req)
		if r.URL.Path != "/chat/completions" || req.Model != "m" || len(req.Modalities) != 2 || !strings.HasPrefix(req.Messages[0].Content, "a boat") {
			t.Errorf("request %s %+v", r.URL.Path, req)
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{
			"content": "", "images": []any{map[string]any{"type": "image_url",
				"image_url": map[string]string{"url": "data:image/png;base64," + base64.StdEncoding.EncodeToString(pic)}}},
		}}}})
	}))
	defer srv.Close()
	img, err := OpenRouter{APIKey: "k", Model: "m", BaseURL: srv.URL}.Generate(context.Background(), "a boat")
	if err != nil || img.Bounds().Dx() != 40 {
		t.Fatalf("err %v", err)
	}
}

func TestPick(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	none := FromEnv(env(nil))
	if c, ok := Pick(none, ""); !ok || c.ID != "placeholder" {
		t.Errorf("nothing set up picked %q", c.ID)
	}
	if err := Check(none, "openai"); err == nil || !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Errorf("Check = %v, want what to set", err)
	}
	if err := Check(none, "dall-e"); err == nil {
		t.Error("unknown provider passed")
	}
	both := FromEnv(env(map[string]string{"GEMINI_API_KEY": "g\r", "LOCAL_IMAGE_URL": "http://sd:1234"}))
	if c, _ := Pick(both, ""); c.ID != "local" || !c.Local {
		t.Errorf("automatic picked %q, want local first", c.ID)
	}
	if c, ok := Pick(both, "gemini"); !ok {
		t.Errorf("gemini not ready: %+v", c)
	} else if g, _ := c.New("").(Gemini); g.APIKey != "g" {
		t.Errorf("key %q, want it trimmed", g.APIKey)
	}
	if _, ok := Pick(both, "openrouter"); ok {
		t.Error("openrouter without a key is ready")
	}
}
