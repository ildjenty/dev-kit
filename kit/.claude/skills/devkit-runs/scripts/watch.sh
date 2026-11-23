#!/usr/bin/env bash
set -uo pipefail

[ "$#" -ge 1 ] || { echo "usage: $0 <run-id> [<run-id> ...]" >&2; exit 2; }

root=$(cd "$(dirname "$0")/../../../.." && pwd)
cd "$root" || exit 1

prev=""
while true; do
  cur=""
  ended=0
  for run in "$@"; do
    status=$(archon workflow get "$run" 2>/dev/null | awk '$1 == "Status:" { print $2; exit }')
    [ -n "$status" ] || status=unknown
    cur="$cur $run=$status"
    case "$status" in
      completed | failed | cancelled | abandoned) ended=$((ended + 1)) ;;
    esac
  done
  if [ "$cur" != "$prev" ]; then
    echo "status:$cur"
    prev="$cur"
  fi
  [ "$ended" -eq "$#" ] && exit 0
  sleep 30
done
