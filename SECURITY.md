# Security policy

## Reporting a vulnerability

Don't open a public issue. Use [Report a vulnerability](https://github.com/Strandgaard96/E-maki/security/advisories/new). Replies are best effort.

## What counts

Anything that leaks audio or transcripts:

- audio or transcripts written to disk, logged, or sent anywhere but the local sidecars (or the external topic model, if picked)
- anything beyond the image prompt reaching the image API
- an API key exposed in logs, responses or the repo

## Scope

The E-maki service, its Dockerfiles and compose files. Problems in whisper.cpp, Ollama or the image API belong with those projects.

The page is plain HTTP on the local network. Don't expose port 8090 to the internet; use a reverse proxy with HTTPS and auth if you must.

Speech can steer the image prompt (prompt injection). That is a known limit, unless it gets transcript text past the checks.
