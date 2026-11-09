#!/bin/bash
# Fails when a pull request carries attribution to an AI assistant.
#   .github/attribution.sh BASE HEAD     (PR_TITLE and PR_BODY in the environment)
# Checks, case-insensitively: every commit message in BASE..HEAD, author and
# committer name/email of each commit, and the PR title and body. An assistant
# named as the subject of a sentence is not attribution and passes.
# Names are assembled from fragments so this file does not match its own rules.
set -u

names='cl''aude|anthr''opic|chatg''pt|open''ai|cop''ilot|cod''ex'
ai="(an? )?(a""i|$names)"
bot=$(printf '\xf0\x9f\xa4\x96')

# Prints one line per violation found in the text on stdin; $1 labels it.
scan_text() {
	local label=$1 text
	text=$(cat)
	printf '%s\n' "$text" | grep -Eiq "^[[:space:]]*co-authored-by:.*($names)" &&
		echo "$label: co-author trailer naming an AI assistant or its maker"
	printf '%s\n' "$text" | grep -Eiq "^[[:space:]]*[a-z-]*session[a-z-]*:[[:space:]]*https?://|($names)[a-z.]*/(code|chat)/(session|[a-z0-9_-]{8,})" &&
		echo "$label: session trailer or link of an AI coding tool"
	printf '%s\n' "$text" | grep -Eiq "(generated|written|created|assisted|authored|made|co-authored)[[:space:]]+(with|by|using)[^a-z0-9]*$ai([^a-z0-9]|$)" &&
		echo "$label: 'generated/written/assisted by' an AI assistant"
	printf '%s\n' "$text" | grep -Fq "$bot" &&
		echo "$label: robot emoji"
	return 0
}

main() {
	local base=${1:?base} head=${2:?head} c
	{
		printf '%s\n' "${PR_TITLE:-}" | scan_text "PR title"
		printf '%s\n' "${PR_BODY:-}" | scan_text "PR body"
		for c in $(git rev-list "$base..$head"); do
			git log -1 --format=%B "$c" | scan_text "commit ${c:0:10} message"
			git log -1 --format='%an <%ae>' "$c" | grep -Eiq "$names" &&
				echo "commit ${c:0:10}: author name or email names an AI assistant or its maker"
			git log -1 --format='%cn <%ce>' "$c" | grep -Eiq "$names" &&
				echo "commit ${c:0:10}: committer name or email names an AI assistant or its maker"
		done
	} | tee /dev/stderr | { ! grep -q .; }
}

main "$@"
