# Laptop test

Talk near a Linux laptop and watch topics turn into pictures.

## Before you start

- Docker with compose v2.
- PulseAudio or PipeWire: `ls $XDG_RUNTIME_DIR/pulse/native` should exist.
- `id -u` should print 1000. If not, set `EMAKI_UID` and `EMAKI_GID` in `.env` to your ids.
- No desktop session: run PipeWire or PulseAudio for the mic's user and `loginctl enable-linger <user>`.

## Run

Install as in [Setup](setup.md#1-install), set `GEMINI_API_KEY` in `.env`, and open <http://localhost:8090/>.

- The first topic needs about a minute of talk.
- Health, money, relationship and work topics are blocked. The log says `topic rejected`. Turn the filters off under Listening > Privacy while testing.
- With `GPU=vulkan` ([Setup](setup.md#gpu)) you can use `WHISPER_MODEL=large-v3-turbo-q5_0`. Go back to `small` if the log shows dropped segments.

## Troubleshooting

- `bind source path does not exist: .../pulse/native`: find the socket with `pactl info | grep "Server String"` and set `PULSE_SOCKET`.
- `miniaudio: Failed to open backend device`: check `EMAKI_UID` matches `id -u`. After restarting the audio server, run `docker compose restart emaki`.
- Every segment is "quiet, skipped": lower the speech level under Listening > Microphone.
- Page doesn't open from another device: open port 8090 in the firewall and leave `PANEL_BIND` empty.
