package imagegen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // Gemini may return JPEG or PNG
	_ "image/png"
	"io"
	"net/http"
	"strings"
	"time"
)

// Gemini generates images with the Gemini API. Only the checked English
// prompt is sent.
type Gemini struct {
	APIKey  string
	Model   string // e.g. "gemini-3.1-flash-image"
	Style   string // appended to every prompt; empty means DefaultStyle
	BaseURL string // default https://generativelanguage.googleapis.com
	Client  *http.Client
}

// Generate asks once more when Gemini answers without an image or with a
// passing error: both happen now and then for an ordinary prompt and a
// valid key (it has answered "API key not valid" for a working key).
func (g Gemini) Generate(ctx context.Context, prompt string) (image.Image, error) {
	img, err := g.generate(ctx, prompt)
	if errors.Is(err, errNoImage) || errors.Is(err, errPassing) {
		time.Sleep(2 * time.Second)
		img, err = g.generate(ctx, prompt)
	}
	return img, err
}

var (
	errNoImage = errors.New("gemini: no image in response")
	errPassing = errors.New("gemini")
)

func (g Gemini) generate(ctx context.Context, prompt string) (image.Image, error) {
	base := g.BaseURL
	if base == "" {
		base = "https://generativelanguage.googleapis.com"
	}
	style := g.Style
	if style == "" {
		style = DefaultStyle
	}
	body, _ := json.Marshal(map[string]any{
		"contents": []any{map[string]any{"parts": []any{map[string]string{"text": prompt + "\n\n" + style}}}},
		"generationConfig": map[string]any{
			"responseModalities": []string{"TEXT", "IMAGE"},
			"imageConfig":        map[string]string{"aspectRatio": "4:3"}, // matches the collage tiles
		},
	})
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent", strings.TrimRight(base, "/"), g.Model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", g.APIKey)
	c := g.Client
	if c == nil {
		c = &http.Client{Timeout: 3 * time.Minute}
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// Google's error message says why (bad key, model, quota); it never
		// echoes the key.
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&e)
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return nil, fmt.Errorf("%w: %s: %s", errPassing, resp.Status, e.Error.Message)
		}
		return nil, fmt.Errorf("gemini: %s: %s", resp.Status, e.Error.Message)
	}
	var out struct {
		PromptFeedback struct {
			BlockReason string `json:"blockReason"`
		} `json:"promptFeedback"`
		Candidates []struct {
			FinishReason string `json:"finishReason"`
			Content      struct {
				Parts []struct {
					Text       string `json:"text"`
					InlineData *struct {
						Data string `json:"data"`
					} `json:"inlineData"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	// An image is a few MB of base64; 64 MB is far above any real answer.
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<20)).Decode(&out); err != nil {
		return nil, err
	}
	for _, cand := range out.Candidates {
		for _, p := range cand.Content.Parts {
			if p.InlineData == nil || p.InlineData.Data == "" {
				continue
			}
			raw, err := base64.StdEncoding.DecodeString(p.InlineData.Data)
			if err != nil {
				return nil, err
			}
			img, _, err := image.Decode(bytes.NewReader(raw))
			return img, err
		}
	}
	// Say why: a block reason, a finish reason, or Gemini's own text reply
	// (which only ever talks about the image prompt).
	var why []string
	if r := out.PromptFeedback.BlockReason; r != "" {
		why = append(why, "blocked: "+r)
	}
	for _, cand := range out.Candidates {
		if cand.FinishReason != "" {
			why = append(why, "finish: "+cand.FinishReason)
		}
		for _, p := range cand.Content.Parts {
			if t := strings.TrimSpace(p.Text); t != "" {
				if len(t) > 200 {
					t = t[:200] + "..."
				}
				why = append(why, fmt.Sprintf("said %q", t))
			}
		}
	}
	return nil, fmt.Errorf("%w (%s)", errNoImage, strings.Join(why, ", "))
}
