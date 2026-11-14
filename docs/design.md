# E-maki design summary

Key decisions (Nov 2026):

- **Privacy boundary:** audio and transcripts stay local and in RAM; only the final English topic / image prompt goes to the cloud. Opt-in exception: an external topic model, picked on the settings page, gets the transcript window.
- **Languages:** Danish (Edda v0.2 fine-tune), Norwegian (NB-Whisper), English (stock Whisper turbo), routed per speech segment.
- **Language:** Go service; speech-to-text and the topic LLM run as HTTP sidecars (whisper.cpp `whisper-server`, Ollama).
- **Display:** 13.3" Spectra 6 (1600x1200) by default; any panel size and portrait through `panel_width`, `panel_height` and `panel_rotation`. E-maki serves Fugleramme's panel protocol (`/frame.png`, `/frame.e6`, `/press`) and reuses its dithering (ported, MIT).
- **Collage:** topics are kept with a decaying weight; recurring topics grow, one-offs fade.
- **Modes:** diary (default: topics are collected through the day and the top ones are painted once, at `diary_at`) and party (live when guests are over). Quiet hours hold new pictures back overnight.
- **Hardware:** postponed. Leading candidate is a Jetson Orin Nano Super brain with ESP32 mic and ESP32 frame clients; differences are handled by config profiles.
