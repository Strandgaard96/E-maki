# Security policy

## Reporting a vulnerability

Please don't open a public issue. Use **[Report a vulnerability](https://github.com/Strandgaard96/E-kiru/security/advisories/new)** on the Security tab.

This is a one-person hobby project, so replies are best effort.

## What counts

E-kiru listens to a room, so anything that could leak audio or transcripts counts as a vulnerability:

- audio or transcript text written to disk, logged, or sent anywhere other than the local sidecars (or the external topic model, when you picked it on the settings page)
- anything beyond the image prompt reaching the image API
- an API key exposed in logs, responses or the repo

## Scope

The E-kiru service, its Dockerfiles and compose files. Problems in whisper.cpp, Ollama or the image API belong with those projects.

The panel page is served as plain HTTP. It is E-kiru's screen, so the compose file makes it reachable from the local network. `PANEL_TOKEN` in `.env` puts it behind a secret path and `PANEL_BIND=127.0.0.1` limits it to the machine itself (see the README). Don't expose port 8090 to the internet; if you need it from somewhere less trusted, put it behind a reverse proxy with HTTPS and authentication.

The image prompt is written by a local LLM from what it heard. Code checks limit what it can carry, but a spoken sentence can still influence it (prompt injection). That is a known limit, not a vulnerability, unless it gets audio or transcript text past the checks.
