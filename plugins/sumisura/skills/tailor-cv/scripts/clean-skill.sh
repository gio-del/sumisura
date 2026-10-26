#!/usr/bin/env bash
# Prints the directory of the clean-user-facing-text skill pinned in
# ../clean-skill.lock (issue #229), downloading it on first use.
#
# The skill's files are not committed to this repo: the lock names the
# upstream repo, the commit and the sha256 of every file. A cached copy is
# used only if every file still matches; otherwise the files are fetched from
# that exact commit into a temp directory, verified, and only then moved into
# the cache. Nothing unverified is ever left where the skill reads from.
#
# Usage (from the repo root):
#   SKILL_DIR="$(plugins/sumisura/skills/tailor-cv/scripts/clean-skill.sh)"
#
# Cache: ${XDG_CACHE_HOME:-~/.cache}/sumisura/clean-user-facing-text/<commit>/
# Exit status: 0 = path printed; 1 = could not fetch or verify (stderr says
# why). The tailor-cv step that calls this is required, so 1 stops the run.
set -euo pipefail

lock="$(cd "$(dirname "$0")/.." && pwd)/clean-skill.lock"
repo="$(sed -n 's/^repo //p' "$lock")"
commit="$(sed -n 's/^commit //p' "$lock")"
case "$repo" in */*) ;; *) echo "clean-skill: no repo in $lock" >&2; exit 1 ;; esac
case "$commit" in *[!0-9a-f]* | "") echo "clean-skill: bad commit in $lock" >&2; exit 1 ;; esac
[ "${#commit}" -eq 40 ] || { echo "clean-skill: commit in $lock is not a full sha" >&2; exit 1; }

sha256() {
	if command -v sha256sum >/dev/null; then sha256sum "$1" | cut -d' ' -f1
	else shasum -a 256 "$1" | cut -d' ' -f1; fi
}

# verify DIR: every file of the lock is in DIR with the pinned hash.
verify() {
	local sum local_path _rest
	while read -r sum local_path _rest; do
		[ -f "$1/$local_path" ] || return 1
		[ "$(sha256 "$1/$local_path")" = "$sum" ] || return 1
	done < <(sed -n 's/^file //p' "$lock")
}

base="${XDG_CACHE_HOME:-$HOME/.cache}/sumisura/clean-user-facing-text"
dest="$base/$commit"
if [ -d "$dest" ] && verify "$dest"; then
	echo "$dest"
	exit 0
fi

command -v curl >/dev/null || { echo "clean-skill: curl is needed to fetch the skill" >&2; exit 1; }
mkdir -p "$base"
tmp="$(mktemp -d "$base/.fetch.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT
while read -r sum local_path upstream_path; do
	case "$local_path$upstream_path" in *..*) echo "clean-skill: bad path in $lock" >&2; exit 1 ;; esac
	mkdir -p "$tmp/$(dirname "$local_path")"
	curl -fsSL --proto '=https' --max-time 60 -o "$tmp/$local_path" \
		"https://raw.githubusercontent.com/$repo/$commit/$upstream_path" ||
		{ echo "clean-skill: could not download $upstream_path from $repo@$commit" >&2; exit 1; }
	[ "$(sha256 "$tmp/$local_path")" = "$sum" ] ||
		{ echo "clean-skill: $upstream_path does not match the sha256 in $lock: not used" >&2; exit 1; }
done < <(sed -n 's/^file //p' "$lock")

rm -rf "$dest"
mv "$tmp" "$dest"
trap - EXIT
echo "$dest"
