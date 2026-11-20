# GitHub settings

[scripts/github-settings.sh](../scripts/github-settings.sh) sets up the GitHub repository the way E-maki expects. It is for the repo owner, and needs [gh](https://cli.github.com/) (logged in with `gh auth login`) and `jq`.

```bash
scripts/github-settings.sh --dry-run   # show what would change
scripts/github-settings.sh             # apply it
```

It reads the current settings first and only changes what differs, so running it again is safe. It sets:

- the topics (and the homepage, if `HOMEPAGE` is set). It never changes the About description.
- Discussions on
- squash merging only, auto-merge allowed, branches deleted after merge
- the `build` label, which builds the E-maki image on the release PR ([Releases](releases.md))
- Dependabot alerts and security updates, private vulnerability reporting, secret scanning and push protection
- two rulesets on the default branch. `main` blocks force pushes and deletion for everyone. `main-prs` lets changes in only through a squash-merged PR that is up to date with the branch and passes the checks in `REQUIRED_CHECKS`; repo admins (you) can still push directly

`--close-stale-dependabot` also closes open Dependabot PRs whose base commit is no longer on the default branch. Dependabot opens a new PR if the update is still needed.

The values live at the top of the script. `REQUIRED_CHECKS` lists the CI jobs. Each one reports on every PR: a job whose files didn't change is skipped, and GitHub counts that as passed. Add only jobs that always report: `build` sums up the Build workflow, so don't add its image jobs.

Releases need a token and a one-time package setting that the script can't set, see [Releases](releases.md).

GitHub's API can't change your account's email privacy, so the script only reminds you: turn on "Keep my email addresses private" at <https://github.com/settings/emails>.
