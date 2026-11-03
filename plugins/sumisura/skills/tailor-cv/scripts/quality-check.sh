#!/usr/bin/env bash
# Quality checks for the tailor-cv skill (issue #104, ADR-0028).
#
# Runs the backend's cvcheck CLI (backend/cmd/cvcheck) — the same Go check
# code the web app's Generate/Render path calls — so a skill-run Generation
# gets the same groundedness, ATS-parsability, page-count and language
# checks an app-run one does. Everything it runs is offline: no backend
# container, no API key, no network.
#
# Usage (from the repo root, like the typst compile it sits next to):
#   scripts/quality-check.sh groundedness --selection output/<slug>/selection.json [--cover-letter <file> [--snippet-ids a,b]] [--json]
#   scripts/quality-check.sh pdf --pdf output/<slug>/cv.pdf --data output/<slug>/data.json [--json]
#
# Exit status is cvcheck's: 0 = ran, nothing flagged; 1 = ran, something
# flagged; 2 = the check could not run. The skill treats none of them as
# fatal. The binary is built from this checkout's backend/ source on each
# call (Go's build cache keeps that fast), so it can never be stale relative
# to the backend; if `go` isn't available, a prebuilt `cvcheck` on PATH is
# used instead, and if neither exists this prints a "check unavailable"
# note and exits 2 rather than failing the pipeline.

set -u

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/../../../../.." && pwd)"

if command -v go >/dev/null 2>&1 && [ -d "$repo_root/backend/cmd/cvcheck" ]; then
  build_dir="$(mktemp -d)"
  trap 'rm -rf "$build_dir"' EXIT
  if ! build_output="$(cd "$repo_root/backend" && go build -buildvcs=false -o "$build_dir/cvcheck" ./cmd/cvcheck 2>&1)"; then
    echo "quality-check: check unavailable: building cvcheck from backend/ failed — continuing without this check." >&2
    echo "$build_output" >&2
    exit 2
  fi
  "$build_dir/cvcheck" "$@"
  exit $?
fi

if command -v cvcheck >/dev/null 2>&1; then
  exec cvcheck "$@"
fi

echo "quality-check: check unavailable: neither Go (to build backend/cmd/cvcheck) nor a prebuilt cvcheck is on PATH — continuing without this check." >&2
exit 2
