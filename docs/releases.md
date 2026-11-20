# Releases

E-maki is published as one image, `ghcr.io/strandgaard96/emaki`, for amd64 and arm64. `docker-compose.yml` names the released version, so a fresh copy of it and `docker compose up -d --build` update it. The whisper and imagegen images are still built on your machine, from the same release on GitHub, because they are compiled for its CPU or GPU.

## How a release happens

1. Every PR into `main` is squash-merged, and its title is a [conventional commit](https://www.conventionalcommits.org/): `fix:` makes the next patch version, `feat:` the next minor one, `feat!:` (a breaking change) the next major one. Other types (`docs:`, `chore:`, `ci:` ...) don't change the version. The PR title check enforces this.
2. After each merge, [release-please](https://github.com/googleapis/release-please) updates one open release PR, "chore(main): release X.Y.Z". It holds the next version, the CHANGELOG and the image tag in `docker-compose.yml`.
3. When you want to release, add the **build** label to the release PR. It builds the E-maki image for both architectures and takes the label off again. Until an image of the PR's current content exists, its `build` check fails, so it can't merge unbuilt. If a new commit lands on it, add the label again.
4. Merging it tags `vX.Y.Z`, writes the GitHub release, and tags the image it already built as `X.Y.Z`, `X.Y` and `latest`. Nothing is built twice.

Other PRs never build the image on their own (`go test` already compiles the code). The **build** label on one test-builds it without publishing.

## One-time setup

These need the repo owner.

- **Release token.** A PR opened with the workflow's own token starts no CI, so the release PR would never pass its checks. Make a [fine-grained token](https://github.com/settings/personal-access-tokens/new) for this repository only, with **Contents: read and write** and **Pull requests: read and write**, and save it as the Actions secret `RELEASE_PLEASE_TOKEN` (Settings > Secrets and variables > Actions).
- **Public image.** After the first release, open the `emaki` package under your profile's Packages, then Package settings > Change visibility > Public, so people can pull it without signing in.
- **Branch rules.** Run `scripts/github-settings.sh` (see [GitHub settings](github-settings.md)).

## Building it yourself

Set `EMAKI_BUILD=1` in `.env` and run `docker compose up -d --build`: E-maki is built from your checkout under the same image name. When you turn it off again, run `docker compose pull emaki` to go back to the released image.
