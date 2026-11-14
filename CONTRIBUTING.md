# Contributing

This is a one-person hobby project, but issues and PRs are welcome.

| You have | Where it goes |
| --- | --- |
| A fix or a doc change | **A PR.** No issue needed. |
| Something is broken | **A [bug report](https://github.com/Strandgaard96/E-maki/issues/new/choose)** |
| An idea, a feature request, a setup question or a frame you built | **[Discussions](https://github.com/Strandgaard96/E-maki/discussions)** |
| A security problem | **The [security policy](SECURITY.md)**, not a public issue |

For a big feature or change, ask in Discussions first, so you don't spend a
weekend on something I won't merge.

Everyone taking part agrees to follow the [code of conduct](CODE_OF_CONDUCT.md).

## Privacy rules

E-maki is an always-on microphone, so these rules always apply:

- Audio is never written to disk. It lives only in RAM.
- Transcript text is never written to disk, logged, or sent off the local
  network. The one exception is the external topic model, which the user has
  to pick on the settings page and which is never a default.
- The only data that may leave the house is the final English image prompt
  sent to the image API.
- Tests, fixtures, logs and issues must not contain real conversations. Use
  synthetic text like `testdata/party.txt`.
- A pull request that changes what is stored, logged or sent over the network
  must say so in its description.

## Development

You need Go (version in `go.mod`) and a C compiler for the audio library.
Docker is only needed for the sidecars.

```sh
go vet ./...
go test ./...     # runs offline: model and API calls have fakes
```

Install the [pre-commit](https://pre-commit.com) hooks once:

```sh
pipx install pre-commit   # or: pip install pre-commit
pre-commit install --hook-type pre-commit --hook-type pre-push
pre-commit run --all-files   # optional first run over the whole tree
```

Each commit then runs, on the files it touches: a
[gitleaks](https://github.com/gitleaks/gitleaks) secret scan, golangci-lint
(gofmt, goimports, go vet, staticcheck), Biome and `tsc` for the settings
page, hadolint, shellcheck, actionlint, yamllint, markdownlint and a 1 MB
limit on new files. `git push` also runs `go test`. The `tsc` hook needs
`npm ci` in `internal/web/ui` first.

CI runs the same hooks in its `lint` job, then only what the change touches:
`go` (tests, with `-race` and `govulncheck` on main), `ui` (typecheck and
build) and `docker` (compose files and the E-maki image). The whisper and sd
images take long to compile, so they only build when their own Dockerfile or
entrypoint changes.

The settings page lives in `internal/web/ui` (Preact, Tailwind and Basecoat).
The built files in `internal/web/static` are committed, so `go build` needs no
Node. After changing the page, rebuild them with Node (current LTS):

```sh
cd internal/web/ui
npm ci
npm run build     # writes ../static/app.js and ../static/app.css
```

CI checks that the committed files match the source.

## Conventions

- Standard library first; add a dependency only when it clearly saves work.
- Model and API calls sit behind small interfaces with fakes, so tests never
  need whisper, Ollama or an API key.
- Never hard-code a device. Hardware differences belong in `configs/`.
- Code ported from Fugleramme keeps its MIT notice (`third_party/fugleramme/`
  and the file headers in `internal/panel`).
- [CLAUDE.md](CLAUDE.md) holds the guidelines AI coding agents follow here;
  they apply to humans too.

> [!NOTE]
> I can only test the hardware I have (a laptop in WSL; Linux is the default setup). If a change touches
> the mic, the GPU setup or the panel, say in the PR what you ran it on.

Write the code yourself or with an LLM, as long as you understand it and can
explain it. PRs are squashed, so the PR
title is the commit message.

## Secrets

Keys go in `.env`, which is git-ignored. Never commit a key, and never paste
one into an issue or log. The pre-commit hook and CI both run gitleaks, so a
key that slips into a commit fails the build. If that happens, revoke the key
first: rewriting history does not un-leak it.

## Licensing

The code is MIT and contributions come in under the same licence. There is no
CLA.
