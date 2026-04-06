#!/bin/sh

set -eu

dest="${1:?usage: install-custom-gcl.sh <dest> <version>}"
version="${2:?usage: install-custom-gcl.sh <dest> <version>}"

tmpdir=$(mktemp -d "${TMPDIR:-/tmp}/custom-gcl.XXXXXX")
trap 'rm -rf "$tmpdir"' EXIT HUP INT TERM

gobin="$tmpdir/bin"
mkdir -p "$gobin"

echo "Building native custom-gcl ${version} for $(go env GOOS)/$(go env GOARCH)"

GOBIN="$gobin" CGO_ENABLED=0 \
	go install "github.com/golangci/golangci-lint/cmd/golangci-lint@${version}"

mkdir -p "$(dirname "$dest")"
mv "$gobin/golangci-lint" "$dest"
chmod +x "$dest"

echo "Installed native custom-gcl to: $dest"
