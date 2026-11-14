# Setup

How to get E-kiru running on the brain, set it up, and show it on a frame. For what to buy, see [Hardware](hardware.md).

## 1. Install

Needs Linux (or Windows with WSL 2) and Docker with compose v2. macOS is untested, and Docker on a Mac can't reach the mic.

```bash
git clone https://github.com/Strandgaard96/E-kiru && cd E-kiru
cp .env.example .env
docker compose up -d --build
```

The first start builds whisper-server and downloads the speech and language models (a few GB), so give it a while. Watch it with `docker compose logs -f`.

Then open <http://localhost:8090/>, or `http://<your-brain>:8090/` from a phone or laptop on your network. That is the settings page. It shows what the frame shows and updates as you talk. In party mode the first topic shows up after about a minute of talk.

Plain `docker compose up` runs everything on the CPU, uses the brain's own mic and makes placeholder pictures. The steps below change that.

### The mic

- **Linux:** works as is with PulseAudio or PipeWire, for the user with id 1000 (`id -u`). For another user id or audio socket, set `EKIRU_UID`, `EKIRU_GID` and `PULSE_SOCKET` in `.env`. Step by step: [Laptop test](laptop-test.md).
- **Windows (WSL):** set `PULSE_SOCKET=/mnt/wslg/PulseServer` in `.env`. See [Windows (WSL)](wsl.md).
- **A mic elsewhere** (a Pi, an ESP32): see [Network microphone](network-mic.md).

### Integrated GPU

An AMD or Intel integrated GPU runs speech-to-text and the topic model through Vulkan. Add a line to `.env` so every `docker compose` command uses it:

```bash
# Linux
COMPOSE_FILE=docker-compose.yml:docker-compose.igpu.yml
# Windows (WSL)
COMPOSE_FILE=docker-compose.yml:docker-compose.igpu-wsl.yml
```

Then `docker compose up -d --build` again. What the host needs is at the top of each file. On WSL, `docker compose config | grep -A3 dockerfile` should show `WHISPER_GPU: vulkan`, and the whisper log names a `Vulkan0` device.

More compose files go in the same line, separated by `:`, for example `docker-compose.yml:docker-compose.igpu-wsl.yml:docker-compose.da-no.yml`.

## 2. Settings in `.env`

`.env` is git-ignored, so keys never end up in the repo. Every setting is explained in [.env.example](../.env.example). The ones most people touch:

| Setting | What for |
| --- | --- |
| `GEMINI_API_KEY` | Real pictures through Gemini. See [Pictures](#3-pictures) |
| `OPENAI_API_KEY`, `OPENROUTER_API_KEY` | Other image services |
| `TZ` | Your time zone, for quiet hours, diary time and the times on the frame |
| `COMPOSE_FILE` | GPU and other extras, see above |
| `WHISPER_MODEL` | `small` (default, fast on a CPU) or `large-v3-turbo-q5_0` (more accurate, wants a GPU) |
| `WHISPER_MODEL_URL` | A ggml fine-tune instead of stock Whisper |
| `WHISPER_DA_MODEL_URL` | Edda for Danish, with `docker-compose.da-no.yml`. See [Languages](#languages) |
| `LLM_MODEL` | The local topic model, `qwen3.5:4b` by default |
| `TOPIC_API_URL`, `TOPIC_API_KEY`, `TOPIC_API_MODEL` | An external topic model. See [External topic model](#external-topic-model) |
| `PANEL_TOKEN` | Puts the page behind a secret address. See [Lock it down](#lock-it-down) |
| `PULSE_SOCKET`, `EKIRU_UID`, `EKIRU_GID` | Where the mic is |

After editing `.env`, run `docker compose up -d` again. Containers whose settings changed are recreated.

### Languages

Stock Whisper is fine for English and weak at Danish and Norwegian. First tick the languages spoken in your home under **E-kiru > Microphone** on the settings page: with one, Whisper skips guessing, and with several, a wrong guess (Danish heard as Swedish) is redone in the first one.

- **One language, Danish or Norwegian:** set `WHISPER_MODEL_URL` to a ggml build of a fine-tune (Edda for Danish, NB-Whisper for Norwegian). That model then handles everything.
- **A mix of Danish, Norwegian and English:** add `docker-compose.da-no.yml`. It runs Edda and NB-Whisper next to stock Whisper and sends each segment to the model for its language. Set `WHISPER_DA_MODEL_URL` to an Edda ggml file you trust, and pin it with `WHISPER_DA_MODEL_SHA256`; Edda's makers publish no ggml file. NB-Whisper has a default. It costs about 1.6 GB more RAM.

### External topic model

If the brain is too slow for a local topic model, E-kiru can use an OpenAI-compatible chat API instead (OpenAI, Gemini, OpenRouter, Groq and others). This sends what was said in the last few minutes to that service. Setting `TOPIC_API_URL`, `TOPIC_API_KEY` and `TOPIC_API_MODEL` only makes it available; pick it under **E-kiru > Topic detection**, which shows a warning while it is on. See [PRIVACY.md](../PRIVACY.md).

## 3. Pictures

E-kiru doesn't come with an image service or a key. Use your own key and pay for your own pictures, or draw them on the brain:

- **Gemini:** get a key in [Google AI Studio](https://aistudio.google.com/apikey) (image generation may need billing switched on) and set `GEMINI_API_KEY`. The model is `GEMINI_IMAGE_MODEL`, `gemini-3.1-flash-image` by default.
- **OpenAI:** set `OPENAI_API_KEY` (model `gpt-image-2` by default). Another service with an OpenAI-style images endpoint works too: also set `OPENAI_BASE_URL`.
- **OpenRouter:** set `OPENROUTER_API_KEY` and the model id in `OPENROUTER_IMAGE_MODEL`.
- **On the brain:** stable-diffusion.cpp with FLUX.2 klein 4B, so nothing leaves the house. A 5.3 GB download and slow without a GPU. See [Local pictures](local-images.md).

With more than one set up, choose under **E-kiru > AI image provider**. Cap spending with the daily picture limit there (50 a day in `docker.json`). Each topic costs one picture when it's first painted; later mentions of it cost nothing. **Make a test picture** in the same place checks that the key works.

## 4. The settings page

Changes apply at once and are kept in the `ekiru-data` volume.

- **Frame:** the live picture, the layout (collage, one picture, or a rolling set of 2 to 9), how often a new picture may go up, quiet hours, captions and picture style.
- **Pictures:** the pictures on the frame (pin, remove, redraw, clear) and the ones waiting to go up, each with the description it was made from.
- **Timeline:** every picture E-kiru made, day by day, with a full-screen viewer. Switch saving off or delete them all there.
- **E-kiru:**
  - *Microphone:* which mic, the speech level meter, languages.
  - *Topic detection:* diary or party mode, how much talk the topic model reads, its instructions, local or external model.
  - *Privacy:* topics that are never painted, and the sensitive-topic filters.
  - *AI image provider:* which one, the daily limit, a test picture.
  - *E-ink frame:* panel size, rotation, how often the frame checks for a new page, what its buttons do.
  - *Security:* a password for the page. Forgot it? Delete `password.json` from the `ekiru-data` volume and restart.
  - *System status:* whether whisper, the topic model and the image provider answer.
- **Logs:** the last lines E-kiru logged, kept in memory.

For settings the page doesn't have (sidecar addresses, segment length, per-language servers), edit the profile in [configs/](../configs/README.md) and run `docker compose restart ekiru`.

## 5. Put it on a frame

E-kiru speaks Fugleramme's panel protocol (`GET /frame.png`, `GET /frame.e6`, `POST /press`), so Fugleramme's frame programs show it unchanged.

**Raspberry Pi with an Inky Impression:** on a Pi with Raspberry Pi OS, install Fugleramme's Inky client with E-kiru's address. It switches on SPI and I2C for the panel; a fresh system may need a reboot after.

```bash
curl -fsSL https://raw.githubusercontent.com/arnegiacomo/fugleramme/main/examples/inky/install.sh \
  | FRAME_URL=http://<your-brain>:8090 bash
```

The four buttons, top to bottom, do this out of the box. Change them under **E-kiru > E-ink frame**:

| Button | Does |
| --- | --- |
| A | Pause or resume listening. While paused the frame says "NOT LISTENING" in the top corner |
| B | Remove the picture that went up last |
| C | Pin that picture so it stays, or let it fade again |
| D | Put the next waiting picture up now |

The change shows at the frame's next refresh.

For a 7.3", 5.7" or 4" Inky, pick the size under **E-kiru > E-ink frame**. Rotate it there too, for portrait.

**ESP32 battery frame and other screens:** see [fugleramme/clients](https://github.com/fugleramme/clients) and [Hardware](hardware.md#battery-frame-esp32).

### Lock it down

Anything on your local network can open the page, change the settings and press the buttons. To stop that, set `PANEL_TOKEN` in `.env` to a long random word (`openssl rand -hex 16`). The page, the picture and the buttons then live under `/<your-token>/` only, and the frame gets `FRAME_URL=http://<your-brain>:8090/<your-token>`. `PANEL_BIND=127.0.0.1` limits the page to the brain itself. Keep port 8090 off the internet.

## Logs

```bash
docker compose logs -f ekiru     # the app
docker compose logs -f whisper   # speech-to-text
docker compose logs -f ollama    # the topic model
```

The E-kiru log has one line per audio segment (level, language, word count, never the words) and one per topic decision. The Logs tab on the settings page shows the same lines. Logs are capped at 30 MB per container.

## Updating

```bash
git pull
docker compose up -d --build
```

Settings, pictures and models are kept in Docker volumes, so they survive an update. Set `LLM_MODEL_UPDATE=1` for one start to fetch a newer upload of the same topic model.

## Troubleshooting

- **No sound, or `miniaudio: Failed to open backend device`:** see the troubleshooting in [Laptop test](laptop-test.md#troubleshooting) or [Windows (WSL)](wsl.md#troubleshooting).
- **Every segment is "quiet, skipped":** lower the speech level under **E-kiru > Microphone**. The meter there shows your mic's level.
- **Topics lag behind or segments are dropped:** Whisper is too slow. Use `WHISPER_MODEL=small` or the GPU.
- **Placeholder tiles instead of pictures:** no image provider is set up, or the key doesn't work. Check **E-kiru > System status** and make a test picture.
- **The page doesn't open from another device:** check the brain's firewall allows port 8090 and that `PANEL_BIND` is empty.
