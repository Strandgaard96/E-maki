# Releases

One image is published: `ghcr.io/strandgaard96/emaki`, for amd64 and arm64. `docker-compose.yml` pins its version. whisper and imagegen are built on your machine from the same release tag.

## How a release happens

1. PRs into `main` are squash-merged. The title is a [conventional commit](https://www.conventionalcommits.org/): `fix:` bumps the patch, `feat:` the minor, `feat!:` the major. Before 1.0, `feat:` bumps the patch and `feat!:` the minor. `docs:`, `chore:` and `ci:` don't bump.
2. [release-please](https://github.com/googleapis/release-please) keeps one release PR open with the next version, the CHANGELOG and the image tag.
   To pick another version, merge a PR whose squash commit ends with `Release-As: X.Y.Z`.
3. The `build` label on the release PR builds the image before merging. Without it the checks pass, and merging builds the image on `main`. Add the label again after a new commit.
4. Merging tags `vX.Y.Z`, writes the GitHub release and tags the image `X.Y.Z`, `X.Y` and `latest`. An image built on the PR is not rebuilt.

Other PRs don't build images. To test-build whisper and imagegen, run the Build workflow by hand.

## One-time setup

- `RELEASE_PLEASE_TOKEN`: a [fine-grained token](https://github.com/settings/personal-access-tokens/new) for this repo with Contents and Pull requests read and write. Without it the release PR runs no CI.
- Make the `emaki` package public after the first release.
- Run `scripts/github-settings.sh` ([GitHub settings](github-settings.md)).

## Build it yourself

Set `EMAKI_BUILD=1` and run `docker compose up -d --build`. To go back, unset it and run `docker compose pull emaki`.
