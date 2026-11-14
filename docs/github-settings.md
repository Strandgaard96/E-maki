# GitHub settings

[scripts/github-settings.sh](../scripts/github-settings.sh) sets up the GitHub repository the way E-maki expects. It is for the repo owner, and needs [gh](https://cli.github.com/) (logged in with `gh auth login`) and `jq`.

```bash
scripts/github-settings.sh --dry-run   # show what would change
scripts/github-settings.sh             # apply it
```

It reads the current settings first and only changes what differs, so running it again is safe. It sets:

- the About description and topics (and the homepage, if `HOMEPAGE` is set)
- Discussions on
- squash merging only, auto-merge allowed, branches deleted after merge
- Dependabot alerts and security updates, private vulnerability reporting, secret scanning and push protection
- a ruleset named `main` on the default branch that blocks force pushes and deletion, and requires the checks in `REQUIRED_CHECKS`

`--close-stale-dependabot` also closes open Dependabot PRs whose base commit is no longer on the default branch. Dependabot opens a new PR if the update is still needed.

The values live at the top of the script. `REQUIRED_CHECKS` is empty for now. CI skips docs-only changes, and a required check that never runs blocks the PR, so only list checks that run on every PR.

GitHub's API can't change your account's email privacy, so the script only reminds you: turn on "Keep my email addresses private" at <https://github.com/settings/emails>.
