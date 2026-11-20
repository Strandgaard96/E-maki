# Setup

What to buy is in [Hardware](hardware.md).

## 1. Install

Needs Linux and Docker with compose v2.

```bash
mkdir e-maki && cd e-maki
curl -fsSLO https://raw.githubusercontent.com/Strandgaard96/E-maki/main/docker-compose.yml
curl -fsSL -o .env https://raw.githubusercontent.com/Strandgaard96/E-maki/main/.env.example
docker compose up -d
```

The first start builds whisper-server and downloads a few GB of models. Follow it with `docker compose logs -f`.

Then open `http://<your-brain>:8090/`.

| Container | Does | Runs |
| --- | --- | --- |
| `emaki` | The app, port 8090 | Always |
| `whisper` | Speech to text | Always |
| `ollama` | Topics | Unless `TOPIC_MODEL=external` |
| `imagegen` | Local pictures | With `LOCAL_IMAGES=1` |

### Mic

PulseAudio or PipeWire works as is for user id 1000. For another user or socket, set `EMAKI_UID`, `EMAKI_GID` and `PULSE_SOCKET`. A mic elsewhere: [Network microphone](network-mic.md).

### GPU

For an AMD or Intel integrated GPU, set `GPU=vulkan` and run `docker compose up -d --build`. The whisper log should name a `Vulkan0` device.

## 2. `.env`

All settings are described in [.env.example](../.env.example). Run `docker compose up -d` after a change.

| Setting | For |
| --- | --- |
| `GEMINI_API_KEY`, `OPENAI_API_KEY`, `OPENROUTER_API_KEY` | Pictures |
| `TZ` | Your time zone |
| `GPU` | `vulkan` for an integrated GPU |
| `WHISPER_MODEL` | `small` (default) or `large-v3-turbo-q5_0` (needs a GPU) |
| `WHISPER_DA_NO` | Danish and Norwegian models |
| `LLM_MODEL` | Local topic model, `qwen3.5:4b` by default |
| `TOPIC_MODEL`, `TOPIC_API_*` | External topic model |
| `PANEL_TOKEN`, `EMAKI_HOSTS` | Access, see [Lock it down](#lock-it-down) |

### Languages

Pick the languages you speak under Listening > Microphone.

- Only Danish or only Norwegian: set `WHISPER_MODEL_URL` to a ggml build of Edda or NB-Whisper.
- A mix with English: set `WHISPER_DA_NO=1` and `WHISPER_DA_MODEL_URL` to an Edda ggml file you trust (pin it with `WHISPER_DA_MODEL_SHA256`). Uses about 1.6 GB more RAM.

### External topic model

For a brain too slow for a local model. It sends the last few minutes of transcript to the service. Set `TOPIC_API_URL`, `TOPIC_API_KEY` and `TOPIC_API_MODEL`, then pick it under Listening > Topics. Add `TOPIC_MODEL=external` to leave Ollama out. See [PRIVACY.md](../PRIVACY.md).

## 3. Pictures

Bring your own key, or draw locally.

- Gemini: key from [Google AI Studio](https://aistudio.google.com/apikey), set `GEMINI_API_KEY`.
- OpenAI: set `OPENAI_API_KEY`. Other OpenAI-style services also need `OPENAI_BASE_URL`.
- OpenRouter: set `OPENROUTER_API_KEY` and `OPENROUTER_IMAGE_MODEL`.
- Local: `LOCAL_IMAGES=1`. See [Local pictures](local-images.md).

Pick one and set the daily limit under System > Image provider.

## 4. Put it on a frame

On a Raspberry Pi with an Inky Impression:

```bash
curl -fsSL https://raw.githubusercontent.com/arnegiacomo/fugleramme/main/examples/inky/install.sh \
  | FRAME_URL=http://<your-brain>:8090 bash
```

Set the panel size, rotation and buttons under Frame > Panel. With 4 buttons, the defaults are:

| Button | Does |
| --- | --- |
| A | Pause or resume listening |
| B | Remove the last picture |
| C | Pin or unpin the last picture |
| D | Show the next picture now |

Other boards: [Hardware](hardware.md#the-frame). Scripts and Home Assistant: [HTTP API](api.md).

### Lock it down

- Set a password under System > Security. It covers the page, not the frame's picture.
- `PANEL_TOKEN` (for example `openssl rand -hex 16`) moves everything under `/<token>/`. The frame then uses `FRAME_URL=http://<your-brain>:8090/<token>`.
- `PANEL_BIND=127.0.0.1` limits the page to the brain.
- Behind a reverse proxy or VPN name, list the name in `EMAKI_HOSTS`.
- Don't expose port 8090 to the internet.

Forgot the password? Delete `password.json` from the `emaki-data` volume and restart.

## Logs

```bash
docker compose logs -f emaki
```

Logs show levels, languages and word counts, never words. The same lines are under System > Logs.

## Update

```bash
curl -fsSLO https://raw.githubusercontent.com/Strandgaard96/E-maki/main/docker-compose.yml
docker compose up -d --build
```

Settings, pictures and models are kept. `LLM_MODEL_UPDATE=1` fetches a newer upload of the topic model on the next start.

## Troubleshooting

- No sound: see [Laptop test](laptop-test.md#troubleshooting).
- Every segment is "quiet, skipped": lower the speech level under Listening > Microphone.
- Topics lag behind: use `WHISPER_MODEL=small` or the GPU.
- Placeholder tiles: no working image key. Make a test picture under System > Image provider.
- Page doesn't open from another device: open port 8090 in the firewall and leave `PANEL_BIND` empty.
