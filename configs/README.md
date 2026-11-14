# Configs

A profile says how E-maki reaches its servers on this machine. Each setting
lives in one place:

- **The settings page** holds everything about what the frame shows,
  listening, topics, privacy and which image provider draws. It saves them
  in its own file (`/data/settings.json` in docker) and nothing else sets
  them. Its defaults are in the code.
- **`.env`** holds keys, models, the GPU and the extras (see
  [.env.example](../.env.example)). System > Services on the page shows
  what it is set to.
- **The profile** holds the rest: the keys below.

Docker compose always uses `docker.json`, and you don't need to touch it.
E-maki reads a profile with `-config`. It is checked at start: a key it does
not know (a typo) or a value out of range stops it with a message. A key
that now lives on the settings page or in `.env` is ignored with a warning
in the log, so an older profile still starts.

| File | For |
| --- | --- |
| `docker.json` | Docker compose |
| `laptop.json` | Running the binary directly next to local sidecars |
| `jetson.json` | Jetson Orin Nano, with Danish and Norwegian speech servers |

## Keys

All need a restart.

| Key | What it does |
| --- | --- |
| `profile` | Name shown under System > Services and in the log |
| `listen` | Panel and web address, e.g. `:8090` |
| `whisper_url`, `whisper_by_lang` | Speech-to-text server, and per-language servers (e.g. `{"da": "http://edda:8081"}`). `WHISPER_URL_DA=http://edda:8081` in the environment adds one too; docker sets these with `WHISPER_DA_NO=1` |
| `llm_url` | OpenAI-compatible topic model server. The model is `LLM_MODEL` in `.env` (default `qwen3.5:4b`), and `LLM_REASONING` is sent as `reasoning_effort` (default `none`; set it empty for models that don't think, like Gemma) |
| `segment_seconds` | Audio per transcription call |
| `mic_timeout_seconds` | An open microphone that sends no audio this long is reopened, e.g. an unplugged USB mic. Default 5 |
| `status_check_seconds` | How often System status checks that whisper and the local topic model answer. Default 10 |
| `test_picture_prompt` | What "Make a test picture" draws. Default: a cartoon teapot |
| `test_pictures_per_day` | Most test pictures a paid image provider makes in a day. Default: 10. Pictures made on this machine are not counted |
| `image_styles` | Your own picture styles for the settings page, like `[{"name": "Woodcut", "prompt": "..."}]`. One with a preset's name replaces it |
| `timeline_thumb_px`, `timeline_jpeg_quality`, `timeline_row_px` | History: thumbnail size (longest side, default 480), JPEG quality of saved pictures (default 90), and row height on the tab (default 220; a phone gets about half) |

Without docker, the network microphone is `NET_MIC_LISTEN=:10700` in the
environment (see [docs/network-mic.md](../docs/network-mic.md)).

## Example: your own picture styles

The panel only has black, white, red, yellow, blue and green, so ask for flat areas of those and no text. The built-in presets end with the same rules.

```json
"image_styles": [
  {"name": "Woodcut", "prompt": "Make it a Japanese woodblock print with bold black outlines and flat areas of pure red, yellow, blue and green on white. No gradients or text."}
]
```
