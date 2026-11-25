#!/usr/bin/env bash
set -uo pipefail

root=$(cd "$(dirname "$0")/../../../.." && pwd)
cd "$root" || exit 1
exec devkit watch "$@"
