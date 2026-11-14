# E-maki

A picture frame that listens to the room and paints what you talk about. Speech-to-text and topic spotting run locally, each topic becomes a cartoon, and the cartoons fill a collage on a colour e-ink panel, a TV or any screen.

<p align="center">
  <img src="docs/collage-example.png" width="640"
       alt="The settings page preview of an E-maki page, in its dark frame, dithered to six e-ink colours, drawn as black ink with red accents: a big picture of red fishing cabins under steep mountains, captioned 'Cabin trip to Lofoten', surrounded by eight smaller drawings for topics like an escaped cat, a sourdough starter, a heat pump, a hot dog, a robot vacuum and a ferry">
  <br>
  <em>An example page as the settings page previews it, laid out and dithered by E-maki's own code for a 13.3" Spectra 6 panel. The topics are made up, and the pictures are hand-drawn stand-ins for the "Ink and red" style.</em>
</p>

<p align="center">
  <a href="https://github.com/Strandgaard96/E-maki/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/Strandgaard96/E-maki/ci.yml?branch=main&style=flat-square&label=CI" alt="CI"></a>
  <a href="#license"><img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="License: MIT"></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/Strandgaard96/E-maki?style=flat-square" alt="Go version"></a>
  <a href="#built-with-claude-code"><img src="https://img.shields.io/badge/built%20with-Claude%20Code-8A2BE2?style=flat-square" alt="Built with Claude Code"></a>
</p>

> [!WARNING]
> Early and experimental. The whole pipeline runs end to end in Docker on a laptop, but no complete frame has been built yet. Expect rough edges and breaking changes.

## Built with Claude Code

> [!WARNING]
> The project is vibe-coded and should be treated as such.

Claude Code wrote most of the code. I steered the design, tested it on my own laptop and decided what to keep. The guidelines it works under are in [CLAUDE.md](./CLAUDE.md), including the privacy rules every change has to follow.

## Inspiration

I'm a hobbyist, and I fell for [Fugleramme](https://github.com/arnegiacomo/fugleramme): an e-ink frame that shows a collage of the birds [BirdNET-Go](https://github.com/tphakala/birdnet-go) hears in the garden. I wanted the same idea for the people in the room, a frame that slowly fills with what we have been talking about. The topics a small local model picks out of a dinner conversation are often funny, and that is half the point.

## The name

An emaki (絵巻) is a Japanese illustrated handscroll that tells a story in pictures as you unroll it, which is roughly what the frame does with a conversation. The *e* (絵) means picture, and it also stands for e-ink.

## How it works

1. **Listen.** A mic records the room in 15-30 second segments. Audio stays in memory and is never saved.
2. **Transcribe.** [whisper.cpp](https://github.com/ggml-org/whisper.cpp) turns each segment into text on your own machine. It can send Danish and Norwegian to fine-tuned models that you point it at ([Languages](docs/setup.md#languages)).
3. **Spot the topic.** About once a minute a local LLM in [Ollama](https://github.com/ollama/ollama) reads the last few minutes and names the topic in English, as specifically as it can ("Is a hot dog a sandwich?", not "food").
4. **Paint it.** A new topic becomes a short image prompt, which is sent to an image API with your own key. This prompt is the only thing about your conversations that leaves your network. Or draw the pictures locally with [stable-diffusion.cpp](https://github.com/leejet/stable-diffusion.cpp), and nothing leaves at all.
5. **Show it.** The picture joins the collage. Topics that keep coming up grow, one-offs fade. The page is dithered for a Spectra 6 e-ink panel and served over HTTP.

Two modes: **Continuous** paints a topic as soon as it comes up, for when guests are over. **Diary** buys no pictures during the day. It collects the topics that came up at least twice, and once a day (`diary_at`, 07:00 by default) paints the top few, so you wake up to yesterday on the wall and the frame refreshes once a day. Diary is meant for a frame on the wall; `jetson.json` uses it. The Docker and laptop profiles use Continuous mode (`"mode": "party"` in the file) so you see results on the first run. Switch on the settings page.

> [!TIP]
> E-maki is a single Go binary. Speech-to-text and the LLM run as separate containers it talks to over HTTP, and differences between machines live in [config profiles](configs/). It runs on a laptop today, with integrated GPU support through Vulkan. The page is a plain PNG, so any browser can show it.

## Privacy

E-maki is an always-on microphone, so:

- Audio lives only in RAM and is never written to disk.
- Transcripts live only in an in-memory rolling window. They are never written to disk or logged, and they stay on your network unless you switch on an external topic model (below). Logs show sound levels, languages and word counts, not words.
- Speech-to-text and the topic LLM run on your own machine, on a Docker network with no route to the internet. Their models are downloaded by separate one-shot containers that never see audio or text.
- With the local topic model, the only thing about your conversations that leaves the house is the short English image prompt, sent with your own key. Without a key nothing does. (The first start downloads the models, but sends nothing about you.) A small local LLM writes the prompt, and code checks it before it goes out (no numbers, no quotes from the transcript, a length limit). It can still slip in a first name or a place now and then. Set `block_names` in the config profile to also refuse prompts that repeat a name it heard.
- Optional, off by default: an external topic model (OpenAI, Gemini, OpenRouter, Groq or any OpenAI-compatible chat API) instead of the local one, for when your machine is too slow for a local LLM. This sends the transcript window, what was said in the last few minutes, to that service. Set `TOPIC_API_KEY` and `TOPIC_API_MODEL` in `.env` and pick it on the settings page under Listening > Topics, which shows a warning while it is on.
- Sensitive topics (health, money, relationships, confidential work) are blocked before any image request, with `block_sensitive` in the config profile. A blocked topic gets no image and no tile, and its label is not logged. All four are blocked in every shipped profile. The local LLM makes the call, so it will sometimes get it wrong.
- The page itself shows what was talked about, and any device on your local network can open it. Set `PANEL_TOKEN` in `.env` to put it behind a secret address; see [Lock it down](docs/setup.md#lock-it-down).

Tell the people in the room that it is listening. [PRIVACY.md](PRIVACY.md) lists everything E-maki keeps, logs and sends.

## Build one

> [!NOTE]
> Only the laptop setup has been run so far. The frame parts below are the ones Fugleramme uses, but no E-maki frame has been built yet, and the dithering and `.e6` output are only checked against Fugleramme's in tests. If you try it, please share how it went in [Discussions](https://github.com/Strandgaard96/E-maki/discussions).

| Part | What works | Notes |
| --- | --- | --- |
| **Brain** | A Linux computer that runs Docker: a laptop, a mini PC, a homelab box. Windows works through WSL | An integrated GPU makes speech-to-text about 7x faster. A Jetson Orin Nano Super is the plan for a small build, untested |
| **Mic** | The brain's own mic or a USB mic | Or a Pi or an ESP32 that streams over Wi-Fi |
| **AI image provider** | Your own Gemini, OpenAI or OpenRouter key, or a local model | Optional: without one you get coloured placeholder tiles |
| **Screen** | Any browser to start with | For the real thing, a 13.3" Spectra 6 e-ink panel ([Inky Impression](https://shop.pimoroni.com/products/inky-impression)) on a Raspberry Pi, or a battery-powered ESP32 board, in an IKEA RÖDALM frame |

Parts, wiring and flashing in **[Hardware](docs/hardware.md)**. Installing, `.env`, the settings page, the frame and logs in **[Setup](docs/setup.md)**.

To try it on a laptop:

```bash
git clone https://github.com/Strandgaard96/E-maki && cd E-maki
cp .env.example .env    # optional: add your own GEMINI_API_KEY
docker compose up --build
```

Then open <http://localhost:8090/> and talk for a minute.

To show it on a Raspberry Pi with an Inky Impression, install Fugleramme's Inky client on the Pi:

```bash
curl -fsSL https://raw.githubusercontent.com/arnegiacomo/fugleramme/main/examples/inky/install.sh \
  | FRAME_URL=http://<your-brain>:8090 bash
```

## Configure

- **`.env`** (copy from [.env.example](.env.example)): image API keys, the Whisper and topic models, the GPU and other extras, the mic socket. See [Setup](docs/setup.md#2-settings-in-env).
- **The settings page** (`http://<your-brain>:8090/`): what the frame shows, and the listening and topic settings. What you change there is saved separately and overrides the profile.
- **`configs/*.json`**: one profile per setup (`docker.json`, `docker-gemma.json`, `laptop.json`, `jetson.json`). Every key is explained in **[Configs](configs/README.md)**.

## Run locally (for development)

Needs Go (version in [go.mod](go.mod)) and a C compiler such as gcc, for the miniaudio library behind the mic capture.

```bash
go run ./cmd/emaki -script testdata/party.txt                  # offline: fake LLM and images, synthetic transcript
go run ./cmd/emaki -config configs/laptop.json -mic            # with whisper-server on :8081 and Ollama on :11434
go run ./cmd/emaki -config configs/laptop.json -wav talk.wav   # the same, from a 16 kHz mono WAV
```

The settings page is at <http://localhost:8090/>, the picture at <http://localhost:8090/frame.png>. Add `-settings settings.json` to keep what the page changes. The code is laid out like this:

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
- **A fix or a doc change:** open a PR, no issue needed
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
