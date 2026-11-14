# Setup

How to get E-maki running on the brain, set it up, and show it on a frame. For what to buy, see [Hardware](hardware.md).

## 1. Install

Needs Linux (or Windows with WSL 2) and Docker with compose v2. macOS is untested, and Docker on a Mac can't reach the mic.

```bash
git clone https://github.com/Strandgaard96/E-maki && cd E-maki
cp .env.example .env
docker compose up -d --build
```

The first start builds whisper-server and downloads the speech and language models (a few GB), so give it a while. Watch it with `docker compose logs -f`.

E-maki prints the settings page's address when it starts:

```bash
docker compose logs emaki | grep "settings page address"
```

It looks like `http://localhost:8090/3f9c…/`. Open it, or put the brain's address in place of `localhost` to open it from a phone or laptop on your network. It shows what the frame shows and updates as you talk. In party mode the first topic shows up after about a minute of talk.

Plain `docker compose up` runs everything on the CPU, uses the brain's own mic and makes placeholder pictures. The steps below change that.

### The mic

- **Linux:** works as is with PulseAudio or PipeWire, for the user with id 1000 (`id -u`). For another user id or audio socket, set `EMAKI_UID`, `EMAKI_GID` and `PULSE_SOCKET` in `.env`. Step by step: [Laptop test](laptop-test.md).
- **Windows (WSL):** set `PULSE_SOCKET=/mnt/wslg/PulseServer` in `.env`. See [Windows (WSL)](wsl.md).
- **A mic elsewhere** (a Pi, an ESP32): see [Network microphone](network-mic.md).

### Integrated GPU

An AMD or Intel integrated GPU runs speech-to-text (and on WSL the topic model) through Vulkan. Set `GPU` in `.env`:

```bash
GPU=vulkan       # Linux
GPU=vulkan-wsl   # Windows (WSL)
```

Then `docker compose up -d --build` again. What the host needs is in [docker/options.yml](../docker/options.yml). `docker compose config | grep WHISPER_GPU` should show `vulkan`, and the whisper log names a `Vulkan0` device. Local pictures use the GPU too.

### Coming from the old compose files

There used to be one compose file per extra (`docker-compose.igpu-wsl.yml` and so on). They are gone, and `.env` decides instead. If your `.env` has a `COMPOSE_FILE` line, delete it and set the matching options:

| Old file | In `.env` now |
| --- | --- |
| `docker-compose.igpu.yml` | `GPU=vulkan` |
| `docker-compose.igpu-wsl.yml` | `GPU=vulkan-wsl` |
| `docker-compose.local-images.yml` (and its igpu files) | `LOCAL_IMAGES=1` |
| `docker-compose.da-no.yml` | `WHISPER_DA_NO=1` |
| `docker-compose.netmic.yml` | `NET_MIC_BIND=0.0.0.0` |

Volumes keep their names, so no model is downloaded again.

## 2. Settings in `.env`

`.env` is git-ignored, so keys never end up in the repo. Every setting is explained in [.env.example](../.env.example). The ones most people touch:

| Setting | What for |
| --- | --- |
| `GEMINI_API_KEY` | Real pictures through Gemini. See [Pictures](#3-pictures) |
| `OPENAI_API_KEY`, `OPENROUTER_API_KEY` | Other image services |
| `TZ` | Your time zone, for quiet hours, diary time and the times on the frame |
| `GPU` | Integrated GPU, see above |
| `WHISPER_MODEL` | `small` (default, fast on a CPU) or `large-v3-turbo-q5_0` (more accurate, wants a GPU) |
| `WHISPER_MODEL_URL` | A ggml fine-tune instead of stock Whisper |
| `WHISPER_DA_NO`, `WHISPER_DA_MODEL_URL` | Edda for Danish and NB-Whisper for Norwegian. See [Languages](#languages) |
| `LLM_MODEL` | The local topic model, `qwen3.5:4b` by default |
| `TOPIC_API_URL`, `TOPIC_API_KEY`, `TOPIC_API_MODEL` | An external topic model. See [External topic model](#external-topic-model) |
| `PANEL_TOKEN`, `PANEL_OPEN` | The page's secret address: your own, or none. Left empty, E-maki makes one up. See [Lock it down](#lock-it-down) |
| `EMAKI_HOSTS` | Extra names the page answers to, for a reverse proxy or a VPN name. See [Lock it down](#lock-it-down) |
| `PULSE_SOCKET`, `EMAKI_UID`, `EMAKI_GID` | Where the mic is |

After editing `.env`, run `docker compose up -d` again. Containers whose settings changed are recreated.

### Languages

Stock Whisper is fine for English and weak at Danish and Norwegian. First tick the languages spoken in your home under **E-maki > Microphone** on the settings page: with one, Whisper skips guessing, and with several, a wrong guess (Danish heard as Swedish) is redone in the first one.

- **One language, Danish or Norwegian:** set `WHISPER_MODEL_URL` to a ggml build of a fine-tune (Edda for Danish, NB-Whisper for Norwegian). That model then handles everything.
- **A mix of Danish, Norwegian and English:** set `WHISPER_DA_NO=1`. It runs Edda and NB-Whisper next to stock Whisper and sends each segment to the model for its language. Set `WHISPER_DA_MODEL_URL` to an Edda ggml file you trust, and pin it with `WHISPER_DA_MODEL_SHA256`; Edda's makers publish no ggml file. NB-Whisper has a default. It costs about 1.6 GB more RAM.

### External topic model

If the brain is too slow for a local topic model, E-maki can use an OpenAI-compatible chat API instead (OpenAI, Gemini, OpenRouter, Groq and others). This sends what was said in the last few minutes to that service. Setting `TOPIC_API_URL`, `TOPIC_API_KEY` and `TOPIC_API_MODEL` only makes it available; pick it under **E-maki > Topic detection**, which shows a warning while it is on. See [PRIVACY.md](../PRIVACY.md).

## 3. Pictures

E-maki doesn't come with an image service or a key. Use your own key and pay for your own pictures, or draw them on the brain:

- **Gemini:** get a key in [Google AI Studio](https://aistudio.google.com/apikey) (image generation may need billing switched on) and set `GEMINI_API_KEY`. The model is `GEMINI_IMAGE_MODEL`, `gemini-3.1-flash-image` by default.
- **OpenAI:** set `OPENAI_API_KEY` (model `gpt-image-2` by default). Another service with an OpenAI-style images endpoint works too: also set `OPENAI_BASE_URL`.
- **OpenRouter:** set `OPENROUTER_API_KEY` and the model id in `OPENROUTER_IMAGE_MODEL`.
- **On the brain:** stable-diffusion.cpp with FLUX.2 klein 4B, so nothing leaves the house. A 5.3 GB download and slow without a GPU. See [Local pictures](local-images.md).

With more than one set up, choose under **E-maki > AI image provider**. Cap spending with the daily picture limit there (50 a day in `docker.json`). Each topic costs one picture when it's first painted; later mentions of it cost nothing. **Make a test picture** in the same place checks that the key works.

## 4. The settings page

Changes apply at once and are kept in the `emaki-data` volume.

- **Frame:** the live picture, the layout (collage, one picture, or a rolling set of 2 to 9), how often a new picture may go up, quiet hours, captions and picture style.
- **Pictures:** the pictures on the frame (pin, remove, redraw, clear) and the ones waiting to go up, each with the description it was made from.
- **Timeline:** every picture E-maki made, day by day, with a full-screen viewer. Switch saving off or delete them all there.
- **E-maki:**
  - *Microphone:* which mic, the speech level meter, languages.
  - *Topic detection:* diary or party mode, how much talk the topic model reads, its instructions, local or external model.
  - *Privacy:* topics that are never painted, and the sensitive-topic filters.
  - *AI image provider:* which one, the daily limit, a test picture.
  - *E-ink frame:* panel size, rotation, how often the frame checks for a new page, what its buttons do.
  - *Security:* a password for the page. Forgot it? Delete `password.json` from the `emaki-data` volume and restart.
  - *System status:* whether whisper, the topic model and the image provider answer.
- **Logs:** the last lines E-maki logged, kept in memory.

For settings the page doesn't have (sidecar addresses, segment length, per-language servers), edit the profile in [configs/](../configs/README.md) and run `docker compose restart emaki`.

## 5. Put it on a frame

E-maki speaks Fugleramme's panel protocol (`GET /frame.png`, `GET /frame.e6`, `POST /press`), so Fugleramme's frame programs show it unchanged.

**Raspberry Pi with an Inky Impression:** on a Pi with Raspberry Pi OS, install Fugleramme's Inky client with E-maki's address. It switches on SPI and I2C for the panel; a fresh system may need a reboot after.

```bash
curl -fsSL https://raw.githubusercontent.com/arnegiacomo/fugleramme/main/examples/inky/install.sh \
  | FRAME_URL=http://<your-brain>:8090 bash
```

The four buttons, top to bottom, do this out of the box. Change them under **E-maki > E-ink frame**:

| Button | Does |
| --- | --- |
| A | Pause or resume listening. While paused the frame says "NOT LISTENING" in the top corner |
| B | Remove the picture that went up last |
| C | Pin that picture so it stays, or let it fade again |
| D | Put the next waiting picture up now |

The change shows at the frame's next refresh.

For a 7.3", 5.7" or 4" Inky, pick the size under **E-maki > E-ink frame**. Rotate it there too, for portrait.

**ESP32 battery frame and other screens:** see [fugleramme/clients](https://github.com/fugleramme/clients) and [Hardware](hardware.md#battery-frame-esp32).

To pause listening from Home Assistant or a script, see the **[HTTP API](api.md)**.

### Lock it down

The page, the picture and the buttons live under `/<token>/` only, so other devices on your network can't open the page, change the settings or press the buttons without the address. On the first start E-maki makes up a random token and keeps it in its data volume (`docker compose exec emaki cat /data/token`). To pick your own, set `PANEL_TOKEN` in `.env` to a long random word (`openssl rand -hex 16`).

A frame gets `FRAME_URL=http://<your-brain>:8090/<token>`, and a network microphone sends the token first (see [network-mic.md](network-mic.md)).

`PANEL_OPEN=1` in `.env` turns the token off: then anything on your network can open the page. `PANEL_BIND=127.0.0.1` limits the page to the brain itself. Keep port 8090 off the internet. The settings page can also have a password, under E-maki > Password.

The page only answers when it is opened by IP address, as `localhost`, or under a local name (`brain`, `brain.local`). That stops a website on the internet from reaching it through your browser. If you open it under another name, such as a reverse proxy's domain or a VPN name, list that name in `EMAKI_HOSTS` in `.env`.

## Logs

```bash
docker compose logs -f emaki     # the app
docker compose logs -f whisper   # speech-to-text
docker compose logs -f ollama    # the topic model
```

The E-maki log has one line per audio segment (level, language, word count, never the words) and one per topic decision. The Logs tab on the settings page shows the same lines. Logs are capped at 30 MB per container.

## Updating

```bash
git pull
docker compose up -d --build
```

Settings, pictures and models are kept in Docker volumes, so they survive an update. Set `LLM_MODEL_UPDATE=1` for one start to fetch a newer upload of the same topic model.

## Troubleshooting

- **No sound, or `miniaudio: Failed to open backend device`:** see the troubleshooting in [Laptop test](laptop-test.md#troubleshooting) or [Windows (WSL)](wsl.md#troubleshooting).
- **Every segment is "quiet, skipped":** lower the speech level under **E-maki > Microphone**. The meter there shows your mic's level.
- **Topics lag behind or segments are dropped:** Whisper is too slow. Use `WHISPER_MODEL=small` or the GPU.
- **Placeholder tiles instead of pictures:** no image provider is set up, or the key doesn't work. Check **E-maki > System status** and make a test picture.
- **The page doesn't open from another device:** check the brain's firewall allows port 8090 and that `PANEL_BIND` is empty.
