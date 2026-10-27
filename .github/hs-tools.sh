#!/bin/sh
# Installs the tools pinned in .hs-tools for linux-amd64 (all of them, or the
# NAMEs given) under $RUNNER_TEMP/hs-tools, refusing any download whose sha256
# differs from the pin, and adds each tool's bin directory to $GITHUB_PATH.
#   .github/hs-tools.sh [NAME...]
set -eu
root=${RUNNER_TEMP:-/tmp}/hs-tools
awk -v p=linux-amd64 '$1 !~ /^#/ && $3 == p' .hs-tools | while read -r name version _ sha url; do
	if [ $# -gt 0 ] && ! printf ' %s ' "$*" | grep -q " $name "; then continue; fi
	dir=$root/$name-$version
	mkdir -p "$dir/out"
	curl -fsSL --proto '=https' -o "$dir/download" "$url"
	echo "$sha  $dir/download" | sha256sum -c --quiet
	case $url in
	*.tar.gz | *.tgz) tar -xzf "$dir/download" -C "$dir/out" ;;
	*) mkdir -p "$dir/out/bin" && cp "$dir/download" "$dir/out/bin/$name" && chmod +x "$dir/out/bin/$name" ;;
	esac
	bin=$(find "$dir/out" -type f -path "*/bin/$name" | head -n1)
	[ -n "$bin" ] || bin=$(find "$dir/out" -type f -name "$name" | head -n1)
	[ -n "$bin" ] || { echo "hs-tools: $name not found in its archive" >&2; exit 1; }
	dirname "$bin" >> "${GITHUB_PATH:-/dev/null}"
	echo "hs-tools: $name $version"
done
