# Laptop test (Linux)

Goal: talk near the laptop and watch topics appear on the page as pictures. On Windows, read [Windows (WSL)](wsl.md) first; the rest of this page then applies.

## Once

1. Docker Engine with compose v2.
2. A desktop session with PulseAudio or PipeWire (any current distro). Check the socket: `ls $XDG_RUNTIME_DIR/pulse/native` should exist.
3. `id -u` should print 1000. If not, put your ids in `.env`: `EMAKI_UID=$(id -u)` and `EMAKI_GID=$(id -g)`, written out as numbers.

On a headless box (no desktop session), run PipeWire or PulseAudio for the user that owns the mic and enable lingering (`loginctl enable-linger <user>`), so the socket exists without a login.

## Run

```sh
git clone https://github.com/Strandgaard96/E-maki && cd E-maki
cp .env.example .env   # optional: set GEMINI_API_KEY for real images via Gemini
docker compose up --build
```

`.env` is git-ignored. Without a key the tiles are coloured placeholders and only the captions show what E-maki understood. With a key, each new topic costs one Gemini image (`gemini-3.1-flash-image`, set `GEMINI_IMAGE_MODEL` to change it), capped per day on the settings page (System > Image provider). Only the English image prompt is sent to Google; audio and transcripts stay on the laptop.

The first start builds whisper-server and downloads the Whisper model (~550 MB) and the topic LLM (`qwen3.5:4b` by default, ~3.4 GB), so give it a while. Then open the settings page at <http://localhost:8090/> (or `http://<laptop-ip>:8090/` from a phone on the same network). It shows the frame and updates as you talk.

The E-maki log shows one line per 15 s segment: its level, the detected language and the word count (never the words), and one line per topic decision.

## What to expect

- Topics need about a minute of talk (4 segments) before the first call to the LLM.
- Health, money, relationship and confidential work topics are held back. The log then says `topic rejected` with the reason. Switch them off on the settings page (Listening > Privacy) to see everything while testing.
- The default speech model is stock Whisper `small`: fast on a laptop CPU, fine for English, weak for Danish. To try a Danish fine-tune, set a ggml model URL, e.g. `WHISPER_MODEL=edda WHISPER_MODEL_URL=<ggml .bin url> docker compose up`.
- By default no GPU is used. With integrated graphics (`GPU=vulkan`, see [Setup](setup.md#integrated-gpu)) there is room for the more accurate `WHISPER_MODEL=large-v3-turbo-q5_0`. If Whisper falls behind (segments dropped in the log), go back to `small`.

## Troubleshooting

- `bind source path does not exist: .../pulse/native`: the audio socket is somewhere else. Find it with `pactl info | grep "Server String"` and set `PULSE_SOCKET` in `.env`.
- `miniaudio: Failed to open backend device`: the container can't use the mic. Check that `EMAKI_UID` matches `id -u`, and restart E-maki if the audio server was restarted (`docker compose restart emaki`).
- Every segment is "quiet, skipped": lower the speech threshold on the settings page (Listening > Microphone). The meter there shows the level of your mic.
- The page does not open from another device: check the laptop's firewall allows port 8090, and that `PANEL_BIND` in `.env` is empty.
