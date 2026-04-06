#!/bin/sh

set -eu

dest="${1:?usage: local-custom-gcl.sh <dest> <version>}"
version="${2:?usage: local-custom-gcl.sh <dest> <version>}"
script_dir=$(CDPATH='' cd -- "$(dirname "$0")" && pwd)

mkdir -p "$(dirname "$dest")"

if command -v custom-gcl >/dev/null 2>&1; then
	ln -sf "$(command -v custom-gcl)" "$dest"
	echo "Using custom-gcl from PATH."
	exit 0
fi

if [ -x "$dest" ]; then
	echo "Using local linter binary: $dest"
	exit 0
fi

if "$script_dir/install-custom-gcl.sh" "$dest" "$version"; then
	echo "Built native custom-gcl: $dest"
	exit 0
fi

echo "error: unable to provision local linter binary" >&2
exit 1
