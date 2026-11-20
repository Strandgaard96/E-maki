# GitHub settings

[scripts/github-settings.sh](../scripts/github-settings.sh) sets up the repository. Needs [gh](https://cli.github.com/) (logged in) and `jq`.

```bash
scripts/github-settings.sh --dry-run   # show changes
scripts/github-settings.sh             # apply
```

It only changes what differs, so it is safe to rerun. It sets:

- topics, and the homepage if `HOMEPAGE` is set. Never the description
- Discussions on
- squash merge only, auto-merge, delete branches after merge
- the `build` label ([Releases](releases.md))
- Dependabot alerts and updates, private vulnerability reporting, secret scanning and push protection
- rulesets on `main`: no force push or delete, and changes only through an up-to-date squash-merged PR with the checks in `REQUIRED_CHECKS`. Admins can still push

`--close-stale-dependabot` closes Dependabot PRs whose base is no longer on `main`.

`REQUIRED_CHECKS` is at the top of the script. Only list jobs that report on every PR. `build` covers the Build workflow, so don't add its image jobs.

Also turn on "Keep my email addresses private" at <https://github.com/settings/emails>.
