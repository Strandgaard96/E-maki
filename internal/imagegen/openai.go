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

	_ "golang.org/x/image/webp" // some services answer in WebP
)

// OpenAI generates images through an OpenAI-style images endpoint
// (POST <BaseURL>/images/generations). OpenAI itself speaks it, and so do
// stable-diffusion.cpp's sd-server and many other services, so the local
// sidecar uses this too, without a key. Only the checked English prompt is
// sent.
type OpenAI struct {
	BaseURL string // up to and including /v1, e.g. https://api.openai.com/v1
	APIKey  string // empty sends no Authorization header (a local server)
	Model   string // empty leaves it out (sd-server serves one model)
	Size    string // "1536x1024"; empty leaves it to the server
	Style   string // appended to every prompt; empty means DefaultStyle
	Client  *http.Client
}

// Generate asks once more when the server is busy or briefly down (a local
// server still loading its model answers 503).
func (o OpenAI) Generate(ctx context.Context, prompt string) (image.Image, error) {
	img, err := o.generate(ctx, prompt)
	if errors.Is(err, errBusy) {
		select {
		case <-time.After(retryWait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		img, err = o.generate(ctx, prompt)
	}
	return img, err
}

var (
	errBusy   = errors.New("images")
	retryWait = 5 * time.Second
)

func (o OpenAI) generate(ctx context.Context, prompt string) (image.Image, error) {
	style := o.Style
	if style == "" {
		style = DefaultStyle
	}
	req := map[string]any{"prompt": prompt + "\n\n" + style, "n": 1}
	if o.Model != "" {
		req["model"] = o.Model
	}
	if o.Size != "" {
		req["size"] = o.Size
	}
	body, _ := json.Marshal(req)
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(o.BaseURL, "/")+"/images/generations", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	if o.APIKey != "" {
		r.Header.Set("Authorization", "Bearer "+o.APIKey)
	}
	c := o.client()
	resp, err := c.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg := errorMessage(resp.Body)
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return nil, fmt.Errorf("%w: %s: %s", errBusy, resp.Status, msg)
		}
		return nil, fmt.Errorf("images: %s: %s", resp.Status, msg)
	}
	var out struct {
		Data []struct {
			B64  string `json:"b64_json"`
			URL  string `json:"url"`
			Text string `json:"revised_prompt"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<20)).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Data) == 0 {
		return nil, errors.New("images: no image in response")
	}
	d := out.Data[0]
	var raw []byte
	switch {
	case d.B64 != "":
		raw, err = base64.StdEncoding.DecodeString(d.B64)
	case d.URL != "":
		raw, err = o.download(ctx, d.URL)
	default:
		return nil, errors.New("images: response has neither b64_json nor url")
	}
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	return img, err
}

func (o OpenAI) client() *http.Client {
	if o.Client != nil {
		return o.Client
	}
	return &http.Client{Timeout: 3 * time.Minute}
}

// download fetches a picture some services link to instead of sending it.
// The link is the service's own and carries no prompt.
func (o OpenAI) download(ctx context.Context, url string) ([]byte, error) {
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return nil, fmt.Errorf("images: unexpected image url %.40q", url)
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := o.client().Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("images: downloading the picture: %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}

// errorMessage reads {"error": {"message": ...}} (OpenAI and most others) or
// {"error": "..."} (sd-server). Neither echoes the key.
func errorMessage(body io.Reader) string {
	var e struct {
		Error json.RawMessage `json:"error"`
	}
	json.NewDecoder(io.LimitReader(body, 1<<20)).Decode(&e)
	var s string
	if json.Unmarshal(e.Error, &s) == nil {
		return s
	}
	var m struct {
		Message string `json:"message"`
	}
	json.Unmarshal(e.Error, &m)
	return m.Message
}
