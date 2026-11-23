#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
bin="$HOME/.local/bin"
if [ "$#" -gt 0 ]; then
  if [ "$#" -ne 2 ] || [ "$1" != "--bin" ] || [ -z "$2" ]; then
    printf 'Usage: scripts/install.sh [--bin DIR]\n' >&2
    exit 2
  fi
  bin="$2"
fi
mkdir -p "$bin"
bin="$(cd "$bin" && pwd)"
cd "$root"
version="$(git describe --always --dirty)"
mise exec -- go build -ldflags "-X main.version=$version" -o "$bin/devkit" ./cmd/devkit
printf 'Installed %s\n' "$bin/devkit"
