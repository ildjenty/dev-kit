#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
version="$(git describe --always --dirty)"
mkdir -p dist
for os in darwin linux; do
  for arch in amd64 arm64; do
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" mise exec -- go build -ldflags "-X main.version=$version" -o "dist/devkit-$os-$arch" ./cmd/devkit
  done
done
