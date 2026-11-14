package imagegen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenRouter generates images through OpenRouter's chat API, which reaches
// many image models (Gemini, GPT Image, FLUX and others) with one key. Only
// the checked English prompt is sent.
type OpenRouter struct {
	APIKey  string
	Model   string // an image model id from openrouter.ai/models
	Style   string // appended to every prompt; empty means DefaultStyle
	BaseURL string // default https://openrouter.ai/api/v1
	Client  *http.Client
}

func (o OpenRouter) Generate(ctx context.Context, prompt string) (image.Image, error) {
	base := o.BaseURL
	if base == "" {
		base = "https://openrouter.ai/api/v1"
	}
	style := o.Style
	if style == "" {
		style = DefaultStyle
	}
	body, _ := json.Marshal(map[string]any{
		"model":        o.Model,
		"messages":     []any{map[string]string{"role": "user", "content": prompt + "\n\n" + style}},
		"modalities":   []string{"image", "text"},
		"image_config": map[string]string{"aspect_ratio": "4:3"}, // matches the collage tiles
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.APIKey)
	c := o.Client
	if c == nil {
		c = &http.Client{Timeout: 3 * time.Minute}
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openrouter: %s: %s", resp.Status, errorMessage(resp.Body))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
				Images  []struct {
					ImageURL struct {
						URL string `json:"url"`
					} `json:"image_url"`
				} `json:"images"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<20)).Decode(&out); err != nil {
		return nil, err
	}
	for _, ch := range out.Choices {
		for _, im := range ch.Message.Images {
			// A data URL: data:image/png;base64,....
			_, data, ok := strings.Cut(im.ImageURL.URL, ";base64,")
			if !ok {
				continue
			}
			raw, err := base64.StdEncoding.DecodeString(data)
			if err != nil {
				return nil, err
			}
			img, _, err := image.Decode(bytes.NewReader(raw))
			return img, err
		}
	}
	why := "no image in response"
	if len(out.Choices) > 0 {
		if t := strings.TrimSpace(out.Choices[0].Message.Content); t != "" {
			if len(t) > 200 {
				t = t[:200] + "..."
			}
			why += fmt.Sprintf(", said %q", t)
		}
	}
	return nil, errors.New("openrouter: " + why)
}
