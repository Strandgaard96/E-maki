# Privacy

E-maki is an always-on microphone. Nothing is recorded. Audio and transcripts stay in memory on your machine. Only a short English picture description leaves the house, unless you turn on an external topic model.

## What happens to the sound

1. The mic is read in 15 to 30 second segments, held in RAM. A network mic sends its audio over your local network unencrypted ([docs/network-mic.md](docs/network-mic.md)).
2. Segments below the speech threshold are dropped untranscribed.
3. The rest goes to whisper-server on your machine, then is dropped. E-maki warns at start if a sidecar address isn't local.
4. The text goes into a rolling window in RAM, a few minutes long.
5. Every few segments the window goes to the local topic model. It returns a topic, an English picture description and a sensitivity label.
6. Sensitive topics, and descriptions that repeat a spoken name, are held back if set on the settings page.
7. The picture description goes to your image API. With `LOCAL_IMAGES=1` it goes to stable-diffusion.cpp on your machine and nothing leaves.

Pausing drops audio before transcription and clears the window.

## External topic model (off by default)

With `TOPIC_API_KEY` and `TOPIC_API_MODEL` set and the model picked under Listening > Topics, **the transcript window is sent to that service every few segments**. What happens to it there is up to the service. Audio stays local and nothing extra is saved. E-maki never picks it on its own, and the settings page and the log warn while it is on.

## What is written to disk

All in the folder given by `-settings` (the `emaki-data` volume in Docker). Without `-settings`, nothing is saved.

| File | Contents |
| --- | --- |
| `settings.json` | Settings, including the never-paint list |
| `password.json` | A salted PBKDF2 hash of the page password |
| `pictures/` | Pictures on the frame and in the queue, with label, description, style and times. Today's count, removed labels, and in diary mode the topics for the next morning. Off with "Keep pictures over a restart" |
| `timeline/` | Every picture made, as JPEG, with label, description, style and time. Off with "Save new pictures" under Pictures > History |

No audio, no transcripts.

## What is logged

Loudness, language, word counts, topic labels, decisions and the picture description sent to the image API. Never transcript text. Sensitive topics are not logged by name.

System > Logs shows the last 1,000 lines, in memory only. The sidecars keep their own logs.

## What the page shows

The frame preview, topic labels, settings and counts. Never transcripts. Anyone on your network can open it unless you set a password.

## Contributors

See [CONTRIBUTING.md](CONTRIBUTING.md#privacy-rules). A PR that changes what is stored, logged or sent must say so.
