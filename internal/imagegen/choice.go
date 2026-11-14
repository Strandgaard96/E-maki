package imagegen

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Choice is an image provider the settings page can offer. API keys and
// addresses come from the environment (.env), never from the page.
type Choice struct {
	ID    string `json:"id"`    // what image_provider names
	Name  string `json:"name"`  // "OpenAI gpt-image-2"
	Local bool   `json:"local"` // nothing leaves the house
	Ready bool   `json:"ready"` // its key or address is set
	Setup string `json:"setup"` // what to put in .env when it is not ready

	make func(style string) Provider
}

// New returns the provider with the art style applied: a preset's name,
// its prompt, or a style typed in (see StylePrompt).
func (c Choice) New(style string) Provider {
	if !c.Ready || c.make == nil {
		return Placeholder{}
	}
	return c.make(StylePrompt(style))
}

// FromEnv lists the providers, ready or not, from environment variables.
// The local one comes first: when it is set up, it is what "automatic"
// picks.
func FromEnv(getenv func(string) string) []Choice {
	env := func(k, def string) string {
		// TrimSpace: a .env saved on Windows ends each value with \r.
		if v := strings.TrimSpace(getenv(k)); v != "" {
			return v
		}
		return def
	}
	var cs []Choice

	localURL := env("LOCAL_IMAGE_URL", "")
	local := Choice{ID: "local", Name: "On this machine (" + env("LOCAL_IMAGE_MODEL", "stable-diffusion.cpp") + ")", Local: true,
		Ready: localURL != "", Setup: "start E-maki with -f docker-compose.local-images.yml added, see docs/local-images.md"}
	if local.Ready {
		size := env("LOCAL_IMAGE_SIZE", "1024x768")
		// A CPU can take minutes for one picture.
		client := &http.Client{Timeout: 20 * time.Minute}
		local.make = func(style string) Provider {
			return OpenAI{BaseURL: strings.TrimRight(localURL, "/") + "/v1", Size: size, Style: style, Client: client}
		}
	}
	cs = append(cs, local)

	gKey, gModel := env("GEMINI_API_KEY", ""), env("GEMINI_IMAGE_MODEL", "gemini-3.1-flash-image")
	cs = append(cs, Choice{ID: "gemini", Name: "Gemini " + gModel, Ready: gKey != "", Setup: "set GEMINI_API_KEY in .env",
		make: func(style string) Provider { return Gemini{APIKey: gKey, Model: gModel, Style: style} }})

	oKey, oModel := env("OPENAI_API_KEY", ""), env("OPENAI_IMAGE_MODEL", "gpt-image-2")
	oSize, oBase := env("OPENAI_IMAGE_SIZE", "1536x1024"), env("OPENAI_BASE_URL", "https://api.openai.com/v1")
	oName := "OpenAI " + oModel
	if u, err := url.Parse(oBase); err == nil && u.Hostname() != "api.openai.com" {
		oName = oModel + " at " + u.Hostname() // another service with the same API
	}
	cs = append(cs, Choice{ID: "openai", Name: oName, Ready: oKey != "", Setup: "set OPENAI_API_KEY in .env",
		make: func(style string) Provider {
			return OpenAI{BaseURL: oBase, APIKey: oKey, Model: oModel, Size: oSize, Style: style}
		}})

	rKey, rModel := env("OPENROUTER_API_KEY", ""), env("OPENROUTER_IMAGE_MODEL", "")
	cs = append(cs, Choice{ID: "openrouter", Name: strings.TrimSpace("OpenRouter " + rModel), Ready: rKey != "" && rModel != "",
		Setup: "set OPENROUTER_API_KEY and OPENROUTER_IMAGE_MODEL in .env",
		make:  func(style string) Provider { return OpenRouter{APIKey: rKey, Model: rModel, Style: style} }})

	// Last, so Pick can fall back on it.
	return append(cs, Choice{ID: "placeholder", Name: "Placeholders (coloured tiles, no pictures)", Local: true, Ready: true,
		make: func(string) Provider { return Placeholder{} }})
}

// Pick returns the choice image_provider names. Empty picks the first one
// that is ready, which is the placeholders when nothing is set up. ok is
// false for an unknown id or one that is not ready; the choice returned
// then is the placeholders.
func Pick(cs []Choice, id string) (c Choice, ok bool) {
	for _, c := range cs {
		if (id == "" && c.Ready) || c.ID == id {
			if !c.Ready {
				break
			}
			return c, true
		}
	}
	return cs[len(cs)-1], false // the placeholders
}

// Check says why image_provider cannot be used: unknown, or not set up.
func Check(cs []Choice, id string) error {
	if _, ok := Pick(cs, id); ok {
		return nil
	}
	for _, c := range cs {
		if c.ID == id {
			return fmt.Errorf("%s is not set up: %s, then restart E-maki", c.Name, c.Setup)
		}
	}
	return fmt.Errorf("unknown image provider %q", id)
}
