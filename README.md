# E-maki

A picture frame that listens to the room and paints what you talk about. Speech-to-text and topic spotting run locally, each topic becomes a cartoon, and the cartoons fill a collage on a colour e-ink panel, a TV or any screen.

<p align="center">
  <img src="docs/collage-example.png" width="640"
       alt="The settings page preview of an E-maki page, in its dark frame, dithered to six e-ink colours, drawn as black ink with red accents: a big picture of red fishing cabins under steep mountains, captioned 'Cabin trip to Lofoten', surrounded by eight smaller drawings for topics like an escaped cat, a sourdough starter, a heat pump, a hot dog, a robot vacuum and a ferry">
  <br>
  <em>An example page, laid out and dithered by E-maki for a 13.3" Spectra 6 panel. The topics are made up.</em>
</p>

<p align="center">
  <a href="https://github.com/Strandgaard96/E-maki/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/Strandgaard96/E-maki/ci.yml?branch=main&style=flat-square&label=CI" alt="CI"></a>
  <a href="#license"><img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="License: MIT"></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/Strandgaard96/E-maki?style=flat-square" alt="Go version"></a>
  <a href="#built-with-claude-code"><img src="https://img.shields.io/badge/built%20with-Claude%20Code-8A2BE2?style=flat-square" alt="Built with Claude Code"></a>
</p>

> [!WARNING]
> Early and experimental. Expect rough edges and breaking changes.

## Built with Claude Code

> [!WARNING]
> The project is vibe-coded and should be treated as such.

Claude Code wrote most of the code. I steered the design and make architecture and tool decisions. The guidelines it works under are in [CLAUDE.md](./CLAUDE.md), including the privacy rules every change has to follow.

## Inspiration

I was heavily inspired by the amazing [Fugleramme](https://github.com/arnegiacomo/fugleramme) project: an e-ink frame that shows a collage of the birds [BirdNET-Go](https://github.com/tphakala/birdnet-go) hears in the garden. I wanted the same idea for conversation topics: a dynamic frame that slowly updates with generated images of conversation topics.

### The name

An emaki (絵巻) is a Japanese illustrated handscroll that tells a story in pictures as you unroll it, and *e* (絵) in Japanese can also mean picture. Thus, E-maki.

## How it works

1. **Listen.** A mic records the room in segments. Audio stays in memory and is never saved.
2. **Transcribe.** [whisper.cpp](https://github.com/ggml-org/whisper.cpp) transcribes each segment into text.
3. **Spot the topic.** About once a minute a local LLM in [Ollama](https://github.com/ollama/ollama) reads the last few minutes and infers the topic.
4. **Paint it.** A new topic becomes a short image prompt, which is sent to an image API. This could be a local image provider with [stable-diffusion.cpp](https://github.com/leejet/stable-diffusion.cpp), or a remote AI image provider.
5. **Display.** The image is fetched by the E-ink driver and displayed. How images are displayed is customized in the E-maki webserver.

> [!TIP]
> E-maki is a single Go binary. Speech-to-text, the topic LLM and local pictures each run in their own container, four at most. Integrated GPUs work through Vulkan. The page is a plain PNG, so any browser can show it.

## Privacy

E-maki is an always-on microphone, so:

- Audio lives only in RAM and is never written to disk.
- Transcripts live only in an in-memory rolling window. They are never written to disk or logged, and they stay on your network unless you switch on an external topic model (below). Logs show sound levels, languages and word counts, not words.
- Speech-to-text and the topic LLM run on your own machine. Their containers download their models on the first start, and send nothing about what they hear.
- With the local topic model, the only thing about your conversations that leaves the house is the short English image prompt, sent with your own API key. A small local LLM writes the prompt, and code checks it before it goes out (no numbers, no quotes from the transcript, a length limit). It can still slip in a first name or a place now and then. Switch on **Spoken names** under Listening > Privacy to also refuse prompts that repeat a name it heard.
- Optional, off by default: an external topic model (OpenAI, Gemini, or any OpenAI-compatible chat API) instead of the local one, for when your machine is too slow for a local LLM. This sends the transcript window, what was said in the last few minutes, to that service. Set `TOPIC_API_KEY` and `TOPIC_API_MODEL` in `.env` and pick it on the settings page under Listening > Topics, which shows a warning while it is on.
- Sensitive topics (health, money, relationships, confidential work) are blocked before any image request, under Listening > Privacy. A blocked topic gets no image and no tile, and its label is not logged. All four are blocked by default. The local LLM makes the call, so it will sometimes get it wrong.
- The page itself shows what was talked about, and any device on your local network can open it. Set a password under System > Security; see [Lock it down](docs/setup.md#lock-it-down).

Tell the people in the room that it is listening. [PRIVACY.md](PRIVACY.md) lists everything E-maki keeps, logs and sends.

## Build one

| Part | What works | Notes |
| --- | --- | --- |
| **Driver** | A Linux computer that runs Docker: a laptop, a mini PC, a homelab server.| An integrated GPU makes speech-to-text about 7x faster |
| **Mic** | Hardware microphone |  |
| **AI image provider** | Your own Gemini, OpenAI or OpenRouter key, or a local model | Optional: without one you get coloured placeholder tiles |
| **Screen** | A 13.3" Spectra 6 e-ink panel ([Inky Impression](https://shop.pimoroni.com/products/inky-impression)) on a Raspberry Pi, or an ESP32 board, in an IKEA RÖDALM frame | No panel yet? The settings page shows the same picture in your browser |

Parts, wiring and flashing in **[Hardware](docs/hardware.md)**. Installing, `.env`, the settings page, the frame and logs in **[Setup](docs/setup.md)**.

To try it, download two files and start it. No clone needed:

```bash
mkdir e-maki && cd e-maki
curl -fsSLO https://raw.githubusercontent.com/Strandgaard96/E-maki/main/docker-compose.yml
curl -fsSL -o .env https://raw.githubusercontent.com/Strandgaard96/E-maki/main/.env.example
docker compose up -d
```

Then open <http://localhost:8090/> and talk for a minute. Add your own `GEMINI_API_KEY` to `.env` for real pictures.

The prebuilt E-maki image comes from ghcr.io. Speech-to-text is compiled on your machine on the first start, and each container downloads its model, so the first start takes a while. Switches in `.env` pick what runs:

| Container | Runs |
| --- | --- |
| `emaki` | Always. The app and its page, port 8090 |
| `whisper` | Always. Speech to text |
| `ollama` | Unless `TOPIC_MODEL=external` |
| `imagegen` | Only with `LOCAL_IMAGES=1` |

E-maki has no frame software of its own. It serves the picture the same way [Fugleramme](https://github.com/arnegiacomo/fugleramme) does, so the programs in [fugleramme/clients](https://github.com/fugleramme/clients) show it on an e-ink board, such as the [XIAO EE02](https://github.com/fugleramme/clients/tree/main/xiao-arduino) for the 13.3" panel. For a Pi with an Inky Impression, see [Put it on a frame](docs/setup.md#5-put-it-on-a-frame).

## Configure

Each setting lives in one place:

- **The settings page** (`http://<your-brain>:8090/`): what the frame shows, listening, topics, privacy and which image provider draws. Hover the ? next to a setting for help.
- **`.env`**: API keys, models, `GPU`, the mic socket. Change it, then run `docker compose up -d`. See [Setup](docs/setup.md#2-settings-in-env).

## Run locally (for development)

Needs Go (version in [go.mod](go.mod)) and a C compiler such as gcc, for the miniaudio library behind the mic capture.

```bash
go run ./cmd/emaki -script testdata/party.txt                  # offline: fake LLM and images, synthetic transcript
go run ./cmd/emaki -config configs/laptop.json -mic            # with whisper-server on :8081 and Ollama on :11434
go run ./cmd/emaki -config configs/laptop.json -wav talk.wav   # the same, from a 16 kHz mono WAV
```

The settings page is at <http://localhost:8090/>, the picture at <http://localhost:8090/frame.png>. Add `-settings settings.json` to keep what the page changes. Set `EMAKI_BUILD=1` in `.env` and run `docker compose up -d --build` to run your checkout in docker. How E-maki reaches its servers lives in [configs/](configs/README.md).

| Package | Role |
|---|---|
| `internal/audio` | microphone capture, network microphone, WAV files, level check |
| `internal/stt` | whisper-server client, per-language routing |
| `internal/topic` | topic LLM over an OpenAI-compatible API, privacy check, offline fake |
| `internal/memory` | topic memory: decay, "seen twice", daily cap, pin and remove |
| `internal/imagegen` | Image providers (Gemini, OpenAI-style APIs including the local sd-server, OpenRouter) and an offline placeholder |
| `internal/collage` | The page at the panel's size: one big tile plus up to 8 smaller tiles with captions |
| `internal/panel` | Fugleramme dithering and `.e6` (ported), `/frame.png`, `/frame.e6`, `/press` |
| `internal/timeline` | the saved pictures behind Pictures > History |
| `internal/web` | the settings page (Preact in TypeScript, Tailwind and Basecoat; source in `internal/web/ui`, built files embedded) |
| `internal/pipeline` | wires it together |

## Contributing

Issues and PRs are welcome.

- **Something is broken:** a [bug report](https://github.com/Strandgaard96/E-maki/issues/new/choose)
- **A question, an idea, or a frame you have built:** [Discussions](https://github.com/Strandgaard96/E-maki/discussions)
- **A fix or a doc change:** open a PR, no issue needed. Its title is a [conventional commit](https://www.conventionalcommits.org/) (`fix: ...`), which [release-please](docs/releases.md) turns into the next release
- **A security or privacy problem:** see the [security policy](SECURITY.md), not a public issue

Read the privacy rules in **[Contributing](CONTRIBUTING.md)** first.

## Credits

- [Fugleramme](https://github.com/arnegiacomo/fugleramme) by Arne Giacomo Munthe-Kaas: the bird frame E-maki grew out of. Its Spectra 6 dithering, `.e6` format and panel protocol are ported to Go in `internal/panel`.
- [BirdNET-Go](https://github.com/tphakala/birdnet-go) by tphakala: the always-on local listener that classifies what it hears, and the idea of running on whatever hardware you have, integrated GPUs included.
- [whisper.cpp](https://github.com/ggml-org/whisper.cpp) and [Ollama](https://github.com/ollama/ollama), which do the speech and language work, and [stable-diffusion.cpp](https://github.com/leejet/stable-diffusion.cpp) for local pictures.

E-maki is not affiliated with any of them.

## License

- Code: MIT. See [`LICENSE`](LICENSE).
- [Preact](https://preactjs.com) (MIT), [Tailwind CSS](https://tailwindcss.com) (MIT) and [Basecoat](https://basecoatui.com) (MIT) for the settings page, and Flickr's [justified-layout](https://github.com/flickr/justified-layout) (MIT) and [PhotoSwipe](https://photoswipe.com) (MIT) for its Timeline.
- Code ported from Fugleramme (`internal/panel`): MIT, Copyright (c) 2026 Arne Giacomo Munthe-Kaas. See [`third_party/fugleramme/LICENSE`](third_party/fugleramme/LICENSE).
- Sidecars ([whisper.cpp](https://github.com/ggml-org/whisper.cpp), [Ollama](https://github.com/ollama/ollama), [stable-diffusion.cpp](https://github.com/leejet/stable-diffusion.cpp)): MIT. Built or pulled as containers, not bundled.
- Models (Whisper and Qwen by default, plus Edda, NB-Whisper, Gemma or FLUX.2 klein if you pick them): downloaded on first start and covered by their own licences. Check each model card before you use one.
- Generated images: made with your own API key and covered by your image provider's terms, or made locally and covered by the model's licence.
