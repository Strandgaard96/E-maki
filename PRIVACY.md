# Privacy

E-maki is an always-on microphone. This is what it does with what it hears.
The short version: **nothing is recorded.** Audio and what was
said stay in memory on your own machine, and only a short English picture
description leaves the house. The one exception is an external topic model,
which you have to switch on yourself (see below).

## What happens to the sound

1. The microphone is read in segments (`segment_seconds`, 15 to 30 seconds).
   Each segment is held in RAM only.
   With the optional network microphone (off by default, see
   [docs/network-mic.md](docs/network-mic.md)), the audio first crosses your
   local network, unencrypted, from the sender to E-maki.
2. A segment quieter than the speech threshold on the settings page is dropped
   without being transcribed.
3. The rest is sent to your speech-to-text sidecar (whisper-server) and then
   dropped. In docker the sidecars sit on a network with no route to the
   internet, and E-maki warns at start if a sidecar address does not look
   local.
4. The text goes into a rolling window in RAM (`window_segments` long, a few
   minutes). Older text falls out of it and is gone.
5. Every few segments the window is sent to your local topic model (Ollama or
   llama-server), which answers with a topic, an English picture description
   and a sensitivity label. If you switch on an external topic model (see
   below), the window goes to that service instead.
6. Topics marked sensitive, or descriptions that repeat a name that was said,
   are held back if you chose so on the settings page. Nothing about them
   leaves the house.
7. Only the English picture description is sent to your image API (Gemini,
   OpenAI or OpenRouter, with your own key). That is the one thing that leaves
   your network. With local pictures (`LOCAL_IMAGES=1`) it
   goes to stable-diffusion.cpp on your own machine instead, which has no
   route to the internet, and nothing leaves at all.

Pausing on the settings page drops audio before it is transcribed and clears
the window.

## External topic model (off by default)

You can let a service outside your network find the topics instead of the
local model: OpenAI, Gemini, OpenRouter, Groq or anything else with an
OpenAI-compatible chat API. Set `TOPIC_API_KEY` and `TOPIC_API_MODEL` in
`.env`, restart, and pick it under E-maki > Topic detection.

This breaks the short version above. With it on, **the transcript window
(what was said in the last few minutes) is sent to that service every few
segments**, and what happens to it there is up to the service and its terms.
Audio still stays on your machine, nothing extra is written to disk, and the
privacy filters still decide which pictures get made, but by then the service
has read the text. E-maki never picks it on its own, the settings page shows a
warning while it is on, and the log says so at start and when you switch.
Tell the people in the room.

## What is written to disk

| File | What is in it |
| --- | --- |
| `settings.json` (next to `-settings`, in docker the `emaki-data` volume) | The options from the settings page, including the never-paint list you typed. No audio and nothing that was said. |
| `password.json` (same folder) | A salted PBKDF2 hash of the settings page password, if you set one. |
| `token` (same folder) | The random address of the page, made on the first start when `PANEL_TOKEN` is not set. |
| `pictures/` (same folder), on unless you switch "Keep pictures over a restart" off | The pictures on the frame and in the queue, with each one's topic label, English picture description, style and times. Also how many pictures were made today and the labels of topics you removed, so the daily limit and a removal hold over a restart. In diary mode also the topics collected for the next morning, as label, picture description and times without a picture. No audio, no words that were said. |
| `timeline/` (same folder), on unless you switch it off | Every picture made since, as a JPEG with a thumbnail, and its topic label, English picture description, style and time, for the Timeline tab. No audio, no words that were said. |

That is all. No audio, no transcripts, no recordings. "Keep pictures over a
restart" is on by default, so the frame comes back as it was. Turned off, the
topics and pictures live in memory and are gone when E-maki restarts, and the
`pictures/` folder's files are deleted.

The timeline is on by default, so pictures pile up in `timeline/` until you
delete them. Switch "Save new pictures" off on the Timeline tab to stop that;
pictures already saved stay until you press "Delete all" there. Without a
settings file (no `-settings`) nothing is saved.

## What is logged

E-maki's log (stdout, so `docker compose logs`) has segment loudness, language
and word counts, the topic labels it found, what it decided about each, and the
picture description sent to the image API. It never logs transcript text, and
a topic held back as sensitive is not logged by name.

The settings page has a Logs tab that shows the last 1,000 of these lines. They
are kept in memory only, behind the page's password when one is set, and are
gone when E-maki stops. The Frame and Timeline tabs also show each picture's description, style
and when it was made. The microphone card shows device names and a loudness number, never audio.

The sidecars keep their own logs. E-maki can't control those, so check
whisper-server's, Ollama's and sd-server's logging if that matters to you.

## What the settings page shows

The frame preview with its topic labels, the settings, and counts (segments
heard, topics found, topics held back). It never shows transcripts. It lives at a private address, so only devices that have the address can open it; a password can be set on top. See [SECURITY.md](SECURITY.md).

## Compared with recording apps

Apps like BirdNET-Go save short audio clips to disk when they detect
something, and filter out clips with human voices. E-maki has no clip saving
at all, so there is nothing to filter or delete.

## Rules for contributors

These are in [CONTRIBUTING.md](CONTRIBUTING.md#privacy-rules): audio and
transcripts never touch the disk, logs or anything off the local network, and a
pull request that changes what is stored, logged or sent must say so.
