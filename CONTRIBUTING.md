# Contributing

Issues and PRs are welcome.

| You have | Where it goes |
| --- | --- |
| A fix or a doc change | A PR. No issue needed |
| A bug | A [bug report](https://github.com/Strandgaard96/E-maki/issues/new/choose) |
| An idea, a question or a frame you built | [Discussions](https://github.com/Strandgaard96/E-maki/discussions) |
| A security problem | The [security policy](SECURITY.md) |

Ask in Discussions before a big change. Follow the [code of conduct](CODE_OF_CONDUCT.md).

## Privacy rules

- Audio is never written to disk. It lives only in RAM.
- Transcript text is never written to disk, logged, or sent off the local network. The only exception is the external topic model, which the user must pick on the settings page and which is never a default.
- Only the English image prompt may leave the house.
- No real conversations in tests, fixtures, logs or issues. Use synthetic text like `testdata/party.txt`.
- A PR that changes what is stored, logged or sent must say so.

## Development

Needs Go (see `go.mod`) and a C compiler.

```sh
go vet ./...
go test ./...     # offline, with fakes
```

Pre-commit hooks:

```sh
pipx install pre-commit
pre-commit install --hook-type pre-commit --hook-type pre-push
```

They run gitleaks, golangci-lint, Biome, `tsc`, hadolint, shellcheck, actionlint, yamllint and markdownlint. Push also runs `go test`. The `tsc` hook needs `npm ci` in `internal/web/ui`.

CI runs the same hooks, then `go`, `ui` and `docker` for what changed. Images are only built on the release PR. See [Releases](docs/releases.md).

The settings page is in `internal/web/ui` (Preact, TypeScript, Tailwind, Basecoat). The built files in `internal/web/static` are committed. After a change:

```sh
cd internal/web/ui
npm ci
npm run build
```

## Conventions

- Standard library first.
- Model and API calls sit behind small interfaces with fakes.
- No hard-coded devices. Hardware differences go in `configs/`.
- Ported Fugleramme code keeps its MIT notice.
- [CLAUDE.md](CLAUDE.md) applies to humans too.
- If a change touches the mic, GPU or panel, say in the PR what you ran it on.
- PRs are squashed. The title is a [conventional commit](https://www.conventionalcommits.org/) (`feat: ...`, `fix(ui): ...`, `docs: ...`).

## Secrets

Keys go in `.env`. Never commit or paste one. If one leaks, revoke it first.

## Licence

MIT. No CLA.
