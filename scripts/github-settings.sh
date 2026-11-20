#!/usr/bin/env bash
# Apply E-maki's GitHub repository settings with the gh CLI.
#
# Run it as a repo admin (the owner). It reads the current settings first and
# only changes what differs, so it is safe to run again.
#
#   scripts/github-settings.sh --dry-run            # show what would change
#   scripts/github-settings.sh                      # apply
#   scripts/github-settings.sh --close-stale-dependabot
#
# Needs gh (logged in with `gh auth login`) and jq. See docs/github-settings.md.
set -euo pipefail

# ---- Settings -------------------------------------------------------------

REPO="${REPO:-Strandgaard96/E-maki}"

DESCRIPTION="A picture frame that listens to the room and paints what you talk about, on a colour e-ink panel. Speech-to-text and topic spotting run locally."
# Empty leaves the homepage as it is.
HOMEPAGE=""
TOPICS=(e-ink spectra6 picture-frame speech-to-text whisper ollama llm
	image-generation collage self-hosted privacy golang)

RULESET_NAME="main"
# Status checks a PR must pass before it can merge, as they are named on a PR
# (the CI job names). Each reports on every PR: a job whose files didn't change
# is skipped, which counts as passed. A check that never runs blocks the merge,
# so add only jobs that always report ("build" sums up the Build workflow).
REQUIRED_CHECKS=(lint go ui docker build pr-title)

# ---- Arguments ------------------------------------------------------------

DRY_RUN=0
CLOSE_STALE=0
for arg in "$@"; do
	case "$arg" in
	--dry-run | -n) DRY_RUN=1 ;;
	--close-stale-dependabot) CLOSE_STALE=1 ;;
	-h | --help)
		sed -n '2,11p' "$0" | sed 's/^# \{0,1\}//'
		exit 0
		;;
	*)
		echo "unknown option: $arg (try --help)" >&2
		exit 2
		;;
	esac
done

for cmd in gh jq; do
	command -v "$cmd" >/dev/null || {
		echo "$cmd is not installed" >&2
		exit 1
	}
done
gh auth status >/dev/null 2>&1 || {
	echo "gh is not logged in: run gh auth login" >&2
	exit 1
}

CHANGES=0

# change DESCRIPTION CMD...: print what changes and run CMD unless --dry-run.
change() {
	local what="$1"
	shift
	CHANGES=$((CHANGES + 1))
	if ((DRY_RUN)); then
		echo "would $what"
	else
		echo "$what"
		"$@" >/dev/null
	fi
}

ok() { echo "ok    $1"; }

# enabled ENDPOINT: true if GET answers 2xx (used by endpoints that answer 404
# for "off").
enabled() { gh api "$1" --silent 2>/dev/null; }

if ((DRY_RUN)); then
	echo "Repository $REPO (dry run, nothing is changed)"
else
	echo "Repository $REPO"
fi
echo

# ---- About, features and merge options ------------------------------------

repo_json=$(gh api "repos/$REPO")

want=$(jq -n \
	--arg description "$DESCRIPTION" \
	--arg homepage "$HOMEPAGE" \
	'{
		description: $description,
		has_discussions: true,
		allow_squash_merge: true,
		allow_merge_commit: false,
		allow_rebase_merge: false,
		delete_branch_on_merge: true,
		allow_auto_merge: true
	} + (if $homepage == "" then {} else {homepage: $homepage} end)')

# Only the fields that differ from what the repo has now.
diff_json=$(jq -n --argjson want "$want" --argjson have "$repo_json" \
	'$want | with_entries(select(.value != $have[.key]))')

if [[ "$diff_json" == "{}" ]]; then
	ok "about, discussions and merge options"
else
	jq -r --argjson have "$repo_json" \
		'to_entries[] | "  \(.key): \($have[.key] | tojson) -> \(.value | tojson)"' <<<"$diff_json"
	change "set about, discussions and merge options" \
		gh api -X PATCH "repos/$REPO" --input - <<<"$diff_json"
fi

# ---- Topics ---------------------------------------------------------------

have_topics=$(gh api "repos/$REPO/topics" --jq '.names | sort | join(" ")')
want_topics=$(printf '%s\n' "${TOPICS[@]}" | sort | paste -sd' ' -)
if [[ "$have_topics" == "$want_topics" ]]; then
	ok "topics"
else
	echo "  topics: ${have_topics:-none} -> $want_topics"
	change "set topics" gh api -X PUT "repos/$REPO/topics" --input - \
		<<<"$(printf '%s\n' "${TOPICS[@]}" | jq -R . | jq -s '{names: .}')"
fi

# ---- Security -------------------------------------------------------------

if enabled "repos/$REPO/vulnerability-alerts"; then
	ok "Dependabot alerts"
else
	change "turn on Dependabot alerts" gh api -X PUT "repos/$REPO/vulnerability-alerts"
fi

if [[ $(gh api "repos/$REPO/automated-security-fixes" --jq .enabled 2>/dev/null) == true ]]; then
	ok "Dependabot security updates"
else
	change "turn on Dependabot security updates" gh api -X PUT "repos/$REPO/automated-security-fixes"
fi

if [[ $(gh api "repos/$REPO/private-vulnerability-reporting" --jq .enabled 2>/dev/null) == true ]]; then
	ok "private vulnerability reporting"
else
	change "turn on private vulnerability reporting" \
		gh api -X PUT "repos/$REPO/private-vulnerability-reporting"
fi

scanning=$(jq -r '[.security_and_analysis.secret_scanning.status,
	.security_and_analysis.secret_scanning_push_protection.status] | join(" ")' <<<"$repo_json")
if [[ "$scanning" == "enabled enabled" ]]; then
	ok "secret scanning and push protection"
else
	change "turn on secret scanning and push protection" \
		gh api -X PATCH "repos/$REPO" --input - <<<'{"security_and_analysis": {
			"secret_scanning": {"status": "enabled"},
			"secret_scanning_push_protection": {"status": "enabled"}}}'
fi

# ---- Ruleset on the default branch ----------------------------------------

# Changes reach the default branch only through a squash-merged PR that is up
# to date with it and passed the checks, so what merges is what was tested.
rules=$(jq -n --args '
	[{type: "deletion"}, {type: "non_fast_forward"}, {
		type: "pull_request",
		parameters: {
			required_approving_review_count: 0,
			dismiss_stale_reviews_on_push: false,
			require_code_owner_review: false,
			require_last_push_approval: false,
			required_review_thread_resolution: false,
			allowed_merge_methods: ["squash"]
		}
	}] +
	(if $ARGS.positional == [] then [] else [{
		type: "required_status_checks",
		parameters: {
			strict_required_status_checks_policy: true,
			required_status_checks: [$ARGS.positional[] | {context: .}]
		}
	}] end)' "${REQUIRED_CHECKS[@]}")

ruleset=$(jq -n --arg name "$RULESET_NAME" --argjson rules "$rules" '{
	name: $name,
	target: "branch",
	enforcement: "active",
	bypass_actors: [],
	conditions: {ref_name: {include: ["~DEFAULT_BRANCH"], exclude: []}},
	rules: $rules
}')

# A short form of a ruleset, to compare the one we want with the one we have.
summary='{enforcement, include: .conditions.ref_name.include,
	rules: [.rules[] | .type] | sort,
	checks: [.rules[] | select(.type == "required_status_checks")
		| .parameters.required_status_checks[].context] | sort,
	up_to_date: [.rules[] | select(.type == "required_status_checks")
		| .parameters.strict_required_status_checks_policy]}'

ruleset_id=$(gh api "repos/$REPO/rulesets" \
	--jq ".[] | select(.name == \"$RULESET_NAME\") | .id" | head -n1)

if [[ -z "$ruleset_id" ]]; then
	change "create ruleset \"$RULESET_NAME\": PRs only, no force pushes or deletion on the default branch${REQUIRED_CHECKS[*]:+, require up-to-date checks: ${REQUIRED_CHECKS[*]}}" \
		gh api -X POST "repos/$REPO/rulesets" --input - <<<"$ruleset"
else
	have=$(gh api "repos/$REPO/rulesets/$ruleset_id" | jq -cS "$summary")
	want=$(jq -cS "$summary" <<<"$ruleset")
	if [[ "$have" == "$want" ]]; then
		ok "ruleset \"$RULESET_NAME\""
	else
		echo "  ruleset: $have"
		echo "        -> $want"
		change "update ruleset \"$RULESET_NAME\"" \
			gh api -X PUT "repos/$REPO/rulesets/$ruleset_id" --input - <<<"$ruleset"
	fi
fi

# ---- Stale Dependabot PRs -------------------------------------------------

# A Dependabot PR is stale when its base commit is no longer in the default
# branch's history (for example after the history was rewritten). Dependabot
# opens a fresh PR if the update is still needed.
if ((CLOSE_STALE)); then
	default_branch=$(jq -r .default_branch <<<"$repo_json")
	while IFS=$'\t' read -r number sha title; do
		[[ -n "$number" ]] || continue
		status=$(gh api "repos/$REPO/compare/$sha...$default_branch" --jq .status 2>/dev/null || echo gone)
		if [[ "$status" == "ahead" || "$status" == "identical" ]]; then
			ok "Dependabot PR #$number ($title)"
		else
			change "close stale Dependabot PR #$number ($title)" \
				gh pr close "$number" --repo "$REPO" --delete-branch \
				--comment "Closing: this PR is based on a commit that is no longer on $default_branch. Dependabot will open a new one if the update is still needed."
		fi
	done < <(gh pr list --repo "$REPO" --author "app/dependabot" --state open --limit 100 \
		--json number,baseRefOid,title --jq '.[] | [.number, .baseRefOid, .title] | @tsv')
fi

# ---- Done -----------------------------------------------------------------

echo
if ((CHANGES == 0)); then
	echo "Everything is already set."
elif ((DRY_RUN)); then
	echo "$CHANGES change(s) to make. Run again without --dry-run to apply."
else
	echo "$CHANGES change(s) made."
fi
echo
echo "Not covered by the API, do it by hand: hide your email in commits at"
echo "https://github.com/settings/emails (Keep my email addresses private, and"
echo "Block command line pushes that expose my email)."
