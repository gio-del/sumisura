#!/usr/bin/env bash
# Calls the Sumisura backend for the tailor-cv skill's tracked-Application
# mode (issue #195).
#
# When the installation runs with an access token (LAN_AUTH_TOKEN, see the
# docs site's LAN mode and Remote access pages), every /api request needs it
# in X-Sumisura-Token — including requests from the machine running the app.
# This wrapper adds the header on its own, so the skill works the same with
# or without a token and the user never passes it by hand.
#
# Usage (from anywhere; paths are resolved from this checkout):
#   scripts/api.sh GET '/api/job-listings?archived=all'
#   scripts/api.sh POST /api/job-listings/<id>/resolve
#   scripts/api.sh PATCH /api/applications/<id>/status '{"status": "tailoring"}'
#
# The token comes from LAN_AUTH_TOKEN in the environment, else from the
# repo root's .env. The backend address is SUMISURA_API_URL, read the same
# way (environment, then .env), default http://127.0.0.1:8080: set it when
# the app runs on another machine, such as a home server. Prints the response body; exits non-zero (curl -f)
# on a connection error or a non-2xx status, like the bare curl it replaces.

set -u

if [ $# -lt 2 ] || [ $# -gt 3 ]; then
  echo "usage: api.sh METHOD PATH [JSON_BODY]" >&2
  exit 2
fi
method="$1"
path="$2"

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/../../../../.." && pwd)"
# from_dotenv NAME: the last uncommented assignment of NAME in the repo
# root's .env, as a shell-sourced .env would resolve it; optional surrounding
# quotes are stripped. The file is read, never sourced.
from_dotenv() {
  [ -f "$repo_root/.env" ] || return 0
  sed -n "s/^[[:space:]]*$1[[:space:]]*=[[:space:]]*//p" "$repo_root/.env" | tail -n 1 | sed -e 's/[[:space:]]*$//' -e 's/^"\(.*\)"$/\1/' -e "s/^'\(.*\)'$/\1/"
}

base_url="${SUMISURA_API_URL:-$(from_dotenv SUMISURA_API_URL)}"
base_url="${base_url:-http://127.0.0.1:8080}"
base_url="${base_url%/}"

token="${LAN_AUTH_TOKEN:-$(from_dotenv LAN_AUTH_TOKEN)}"

args=(-sf -X "$method")
if [ -n "$token" ]; then
  args+=(-H "X-Sumisura-Token: $token")
fi
if [ $# -eq 3 ]; then
  args+=(-H 'Content-Type: application/json' -d "$3")
fi

exec curl "${args[@]}" "$base_url$path"
