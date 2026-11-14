package topic

import (
	"net/url"
	"strings"
)

// External is a topic model on a service outside the house, set up in .env
// (TOPIC_API_KEY, TOPIC_API_MODEL, TOPIC_API_URL). Using it sends the whole
// transcript window off the local network, so it is only used when the
// settings page picks it (topic_provider "external"), never automatically.
type External struct {
	URL       string // the API's base address, e.g. https://api.openai.com/v1
	Key       string
	Model     string
	Reasoning string // sent as reasoning_effort when set
}

// ExternalSetup says what to put in .env when External is not ready.
const ExternalSetup = "set TOPIC_API_KEY and TOPIC_API_MODEL in .env, then run docker compose up -d"

func ExternalFromEnv(getenv func(string) string) External {
	env := func(k, def string) string {
		if v := strings.TrimSpace(getenv(k)); v != "" {
			return v
		}
		return def
	}
	return External{URL: env("TOPIC_API_URL", "https://api.openai.com/v1"), Key: env("TOPIC_API_KEY", ""),
		Model: env("TOPIC_API_MODEL", ""), Reasoning: env("TOPIC_API_REASONING", "")}
}

// Ready reports whether a key and a model are set.
func (e External) Ready() bool { return e.Key != "" && e.Model != "" }

// Name is what the settings page shows: "gpt-x at api.openai.com".
func (e External) Name() string {
	host := e.URL
	if u, err := url.Parse(e.URL); err == nil && u.Hostname() != "" {
		host = u.Hostname()
	}
	return e.Model + " at " + host
}

// Extractor sends the window to the service's /chat/completions.
func (e External) Extractor(prompt string, temperature *float64) OpenAI {
	return OpenAI{Endpoint: strings.TrimRight(e.URL, "/") + "/chat/completions", APIKey: e.Key, Model: e.Model,
		Prompt: prompt, Temperature: temperature, Reasoning: e.Reasoning}
}
