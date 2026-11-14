# Configs

One JSON file per setup. E-kiru reads it with `-config`; docker compose picks
`configs/$EKIRU_CONFIG` (default `docker.json`) and mounts this folder, so
an edit only needs `docker compose restart ekiru`. Secrets and model
choices for docker live in `.env` (see `.env.example`).

The settings page (`http://<your-brain>:8090/`) can change `mode`, `min_speech_dbfs`, `languages`, `window_segments`, `extract_every`, `topic_prompt`, `llm_temperature`, `image_style`, `image_provider`, `block_sensitive`, `block_names`, `never_paint`, `half_life_hours`, `min_confidence`, `daily_image_cap`, the layout keys below, `panel_buttons`, `panel_width`, `panel_height`, `panel_rotation` and `panel_poll_minutes` while E-kiru runs. The sidecar addresses, models and `segment_seconds` need a restart. Started with `-settings <file>` (docker compose uses `/data/settings.json` in the `ekiru-data` volume), it saves them there, and that file overrides the profile on the next start. Delete the file to go back to the profile.

A profile is checked when E-kiru starts: a key it does not know (a typo) or
a value out of range stops it with a message, instead of being ignored.

| File | For |
| --- | --- |
| `docker.json` | Docker compose (default, Qwen 3.5 4B with thinking off) |
| `docker-gemma.json` | Same, for Gemma 4 models |
| `laptop.json` | Running the binary directly next to local sidecars |
| `jetson.json` | Jetson Orin Nano, diary mode, names blocked too |

## Keys

| Key | What it does |
| --- | --- |
| `profile`, `mode` | Name for the log; `party` (show topics at once) or `diary` (collect the day's topics, paint the top ones once a day) |
| `diary_at` | Diary mode: when the day's topics are painted, like `"07:00"` (default). Quiet hours can hold them back further; `new_every_minutes` is not used |
| `listen` | Panel/web address, e.g. `:8090` |
| `whisper_url`, `whisper_by_lang` | Speech-to-text server, and per-language servers (e.g. `{"da": "http://edda:8081"}`). `WHISPER_URL_DA=http://edda:8081` in the environment adds one too |
| `languages` | Languages spoken in the house, as Whisper codes, e.g. `["da", "en"]` (any language Whisper knows). One skips language detection; with several, a guess outside the list is redone in the first one. Empty (default) accepts any language |
| `llm_url`, `llm_model` | OpenAI-compatible LLM server and model. `LLM_MODEL` in the environment overrides the model |
| `topic_prompt` | Replaces the built-in instructions for naming the topic and writing the image prompt (`topic.DefaultPrompt`). The privacy rules and the JSON answer format are always added after it |
| `llm_temperature` | 0 = most predictable, higher = more varied topics. Default 0.2 |
| `llm_reasoning` | Sent as `reasoning_effort`; `"none"` stops thinking models (Qwen 3.5) from spending a minute thinking. Leave out for models that do not think |
| `segment_seconds` | Audio per transcription call |
| `status_check_seconds` | How often System status checks that whisper and the local topic model answer. Default 10 |
| `test_picture_prompt` | What "Make a test picture" draws. Default: a cartoon teapot |
| `net_mic_listen` | Listen for a network microphone, e.g. `":10700"` (see [docs/network-mic.md](../docs/network-mic.md)). Empty is off. `NET_MIC_LISTEN` overrides it. Needs a restart |
| `min_speech_dbfs` | Quieter segments are skipped |
| `window_segments` | How many recent segments the LLM sees (8 x 15 s = 2 min) |
| `extract_every` | Segments between topic calls |
| `half_life_hours`, `min_confidence` | How fast tiles fade; topics below this confidence are ignored |
| `daily_image_cap` | Most image API calls per day |
| `block_sensitive` | Sensitive categories never sent to the image API: `health`, `money`, `relationship`, `confidential_work`. With any category listed, a topic the LLM does not clearly mark as `none` or an allowed category is held back too. Empty sends everything |
| `block_names` | `true` refuses an image prompt that repeats a name heard in the transcript (a word capitalised mid-sentence). It cannot tell people from places or brands, so it also drops "Lofoten" or "Italian". Default `false` |
| `never_paint` | Words or phrases that never become a picture, for example `["work", "Anna"]`. A topic or picture description that contains one (whole words, any case, plural too) is held back. The list is never logged. Default empty |
| `image_style` | Art style added to every image prompt: a preset's name (`Comic`, `Clear line`, `Picture book`, `Linocut`, `Paper cut-out`, `Stained glass`, `Screen print`, `Pop art`, `Ink and red`, `Pixel art`) or a style of your own. Empty is `Comic`. The presets are in `internal/imagegen/styles.go` |
| `image_styles` | Your own presets for the settings page, like `[{"name": "Woodcut", "prompt": "..."}]`. One with a preset's name replaces it, so the first preset is the default. Needs a restart |
| `image_provider` | Who draws the pictures: `local`, `gemini`, `openai`, `openrouter` or `placeholder`. Empty (default) takes the first one set up in `.env`, local first. Keys and addresses stay in `.env` |
| `layout`, `tiles` | `collage` (default: most talked-about biggest, tiles fade), `single` (newest topic only; pin to keep it) or `rolling` (newest first, the oldest drops off). `tiles` is the most pictures, 1 to 9 (default 9) |
| `new_every_minutes` | Least minutes between two new pictures; new topics wait in line. 0 (default) puts each up at once |
| `quiet_from`, `quiet_to` | Quiet hours like `"22:00"` and `"07:00"`: no new pictures go up in between. Times follow `TZ` |
| `newest_big`, `hide_captions` | Collage: the newest topic takes the big tile. Leave the topic names off the pictures |
| `timeline` | `false` stops saving new pictures for the Timeline tab (the tab has the same switch). Default on |
| `timeline_thumb_px`, `timeline_jpeg_quality`, `timeline_row_px` | Timeline: thumbnail size (longest side, default 480), JPEG quality of saved pictures (default 90), and row height on the tab (default 220; a phone gets about half). Need a restart |
| `paused` | `true` ignores the microphone (the settings page sets this) |
| `panel_buttons` | What the frame's buttons do, e.g. `{"A": "pause", "B": "remove", "C": "pin", "D": "next"}` (the default). Actions: `pause` (pause or resume listening), `remove` (take the last picture off), `pin` (pin it, or let it fade again), `next` (put the next waiting picture up), `""` (nothing). A button left out keeps its default |
| `panel_width`, `panel_height` | The e-ink panel's resolution, landscape: `1600`×`1200` for the 13.3" Inky Impression (the default), `800`×`480` for the 7.3", `600`×`448` for the 5.7", `640`×`400` for the 4". The page is drawn at this size, and in portrait when `panel_rotation` is 90 or 270. `/frame.e6` exists for the 7.3" and 13.3" only |
| `panel_rotation`, `panel_poll_minutes` | Passed to the Fugleramme panel clients |

## Example: your own topic prompt

```json
"topic_prompt": "You read a transcript of a home conversation. Name its topic in English in at most 8 words, as specific and funny as you can: the exact object, place or mishap, never a category like 'travel'. Write visual_prompt as one literal cartoon scene of at most 30 words."
```

## Example: your own picture styles

The panel only has black, white, red, yellow, blue and green, so ask for flat areas of those and no text. The built-in presets end with the same rules.

```json
"image_styles": [
  {"name": "Woodcut", "prompt": "Make it a Japanese woodblock print with bold black outlines and flat areas of pure red, yellow, blue and green on white. No gradients or text."}
]
```
