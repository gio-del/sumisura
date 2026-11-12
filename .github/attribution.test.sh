#!/bin/bash
# Tests for attribution.sh: one passing case and one failing case per rule.
# Run: .github/attribution.test.sh
set -u
here=$(cd "$(dirname "$0")" && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
cd "$tmp" && git init -q . && git config commit.gpgsign false
export GIT_AUTHOR_NAME=Dev GIT_AUTHOR_EMAIL=dev@example.org GIT_COMMITTER_NAME=Dev GIT_COMMITTER_EMAIL=dev@example.org
git commit -q --allow-empty -m base
base=$(git rev-parse HEAD)

# Fragments, so this file does not contain the strings it looks for.
C=cl''aude A=anthr''opic I=A''I
bot=$(printf '\xf0\x9f\xa4\x96')
GW="Gener""ated with" WB="Writ""ten by" AB="Assis""ted by" CO="Co-Auth""ored-by"
fail=0

# expect pass|fail NAME: commits $MSG with the identities in the environment;
# PR_TITLE/PR_BODY come from the caller.
expect() {
	local want=$1 name=$2 out rc
	# commit-tree, not commit: no hooks, so a host-wide message hook can't
	# reject the very messages under test.
	local tip
	tip=$(git commit-tree "$base^{tree}" -p "$base" -m "${MSG:-work}") || { echo "FAIL $name (setup)"; fail=1; return; }
	out=$("$here/attribution.sh" "$base" "$tip" 2>&1) && rc=0 || rc=1
	if { [ "$want" = pass ] && [ $rc = 0 ]; } || { [ "$want" = fail ] && [ $rc = 1 ]; }; then
		echo "ok   $name"
	else
		echo "FAIL $name (wanted $want)"; echo "$out"; fail=1
	fi
}

MSG="feat: add thing" PR_TITLE="feat: add thing" PR_BODY="Needs an API key from $A." expect pass "clean, assistant as subject"
MSG="Serve una chiave API di $C per il flusso" expect pass "assistant named as subject in commit"
MSG="feat: x

$CO: $C <noreply@$A.com>" expect fail "co-author trailer"
MSG="feat: x

$CO: Someone <a@$A.com>" expect fail "co-author, maker address"
MSG="feat: x

https://$C.ai/code/session_01AbCdEfGhIjKl" expect fail "session link"
MSG="feat: x

Session-Id: https://example.org/s/123" expect fail "session trailer"
MSG="feat: x

$GW [$C Code](https://x.example)" expect fail "generated with"
MSG="feat: x

$WB $C" expect fail "written by"
MSG="feat: x

$AB an $I" expect fail "assisted by an assistant"
MSG="feat: x $bot" expect fail "robot emoji"
MSG="feat: x" GIT_AUTHOR_NAME="$C" expect fail "author name"
MSG="feat: x" GIT_AUTHOR_EMAIL="noreply@$A.com" expect fail "author email"
MSG="feat: x" GIT_COMMITTER_NAME="$C Code" expect fail "committer name"
MSG="feat: x" GIT_COMMITTER_EMAIL="x@$A.com" expect fail "committer email"
MSG="feat: x" PR_TITLE="$GW $C" expect fail "PR title"
MSG="feat: x" PR_BODY="Body

$bot $WB $C" expect fail "PR body"

exit $fail
