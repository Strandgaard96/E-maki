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
)

func TestGemini(t *testing.T) {
	var buf bytes.Buffer
	png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 40, 30)))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models/gemini-3.1-flash-image:generateContent" || r.Header.Get("x-goog-api-key") != "k" {
			t.Errorf("unexpected request %s key %q", r.URL.Path, r.Header.Get("x-goog-api-key"))
		}
		var req struct {
			Contents []struct{ Parts []struct{ Text string } }
		}
		json.NewDecoder(r.Body).Decode(&req)
		if !strings.HasPrefix(req.Contents[0].Parts[0].Text, "snowy peaks") || !strings.Contains(req.Contents[0].Parts[0].Text, DefaultStyle) {
			t.Errorf("prompt %q", req.Contents[0].Parts[0].Text)
		}
		json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{
			map[string]string{"text": "here you go"},
			map[string]any{"inlineData": map[string]string{"mimeType": "image/png", "data": base64.StdEncoding.EncodeToString(buf.Bytes())}},
		}}}}})
	}))
	defer srv.Close()
	img, err := Gemini{APIKey: "k", Model: "gemini-3.1-flash-image", BaseURL: srv.URL}.Generate(context.Background(), "snowy peaks")
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 40 {
		t.Fatalf("got %v", img.Bounds())
	}
}

func TestGeminiNoImageRetriesAndSaysWhy(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{
			"finishReason": "IMAGE_SAFETY",
			"content":      map[string]any{"parts": []any{map[string]string{"text": "I can't draw that character."}}},
		}}})
	}))
	defer srv.Close()
	_, err := Gemini{APIKey: "k", Model: "m", BaseURL: srv.URL}.Generate(context.Background(), "a cartoon hero")
	if calls != 2 {
		t.Errorf("calls = %d, want 2 (one retry)", calls)
	}
	if err == nil || !strings.Contains(err.Error(), "IMAGE_SAFETY") || !strings.Contains(err.Error(), "can't draw") {
		t.Errorf("err = %v, want the finish reason and Gemini's text", err)
	}
}

func TestGeminiRetriesPassing401(t *testing.T) {
	var buf bytes.Buffer
	png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 3)))
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":{"message":"API key not valid."}}`))
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{
			map[string]any{"inlineData": map[string]string{"data": base64.StdEncoding.EncodeToString(buf.Bytes())}},
		}}}}})
	}))
	defer srv.Close()
	if _, err := (Gemini{APIKey: "k", Model: "m", BaseURL: srv.URL}).Generate(context.Background(), "p"); err != nil || calls != 2 {
		t.Fatalf("err %v after %d calls, want an image after one retry", err, calls)
	}
}
