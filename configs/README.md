# Configs

A profile tells E-maki where its servers are. Pass it with `-config`. Docker uses `docker.json`; you don't need to edit it.

The settings page and `.env` hold everything else. An unknown key or a bad value stops E-maki at start. A key that moved to the page or `.env` is ignored with a warning.

| File | For |
| --- | --- |
| `docker.json` | Docker compose |
| `laptop.json` | The binary with local sidecars |

## Keys

A change needs a restart.

| Key | What it does |
| --- | --- |
| `profile` | Name shown under System > Services and in the log |
| `listen` | Panel and web address, e.g. `:8090` |
| `whisper_url`, `whisper_by_lang` | Speech-to-text server, and one per language (e.g. `{"da": "http://edda:8081"}`). `WHISPER_URL_DA` in the environment does the same |
| `llm_url` | OpenAI-compatible topic model server. The model is `LLM_MODEL` in `.env` |
| `segment_seconds` | Audio per transcription call |
| `mic_timeout_seconds` | Reopen a mic that sends nothing for this long. Default 5 |
| `status_check_seconds` | How often to check the servers answer. Default 10 |
| `test_picture_prompt` | What "Make a test picture" draws. Default: a cartoon teapot |
| `test_pictures_per_day` | Test picture limit for paid providers. Default 10 |
| `image_styles` | Extra picture styles, see below. One with a preset's name replaces it |
| `timeline_thumb_px`, `timeline_jpeg_quality`, `timeline_row_px` | History thumbnail size (480), JPEG quality (90) and row height (220) |

## Picture styles

The panel has black, white, red, yellow, blue and green. Ask for flat areas of those and no text.

```json
"image_styles": [
  {"name": "Woodcut", "prompt": "Make it a Japanese woodblock print with bold black outlines and flat areas of pure red, yellow, blue and green on white. No gradients or text."}
]
```
