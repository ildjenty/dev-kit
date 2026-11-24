#!/usr/bin/env bash
set -uo pipefail

# Usage: scripts/test-kit.sh [workflow ...]  (no names: every fixture)

root="$(cd "$(dirname "$0")/.." && pwd)"
mkdir -p "$root/.local/tmp"
temp="$(mktemp -d "$root/.local/tmp/test-kit.XXXXXX")"
trap 'rm -rf "$temp"' EXIT
mkdir -p "$temp/bin"
if ! (cd "$root" && mise exec -- go build -o "$temp/bin/devkit" ./cmd/devkit); then
  exit 1
fi
if ! (cd "$root" && mise exec -- go build -o "$temp/bin/devkit-fixture" ./cmd/devkit-fixture); then
  exit 1
fi
export PATH="$temp/bin:$PATH"
export ARCHON_HOME="$temp/archon-home"
export ARCHON_TELEMETRY_DISABLED=1
mkdir -p "$ARCHON_HOME"
printf 'store: files\n' > "$ARCHON_HOME/config.yaml"
plugins="${KIT_ARCHON_PLUGINS:-$HOME/.archon/plugins}"
if [ -d "$plugins" ]; then
  ln -s "$plugins" "$ARCHON_HOME/plugins"
else
  printf 'WARN: no Archon provider plugins at %s; validation of AI nodes may fail\n' "$plugins"
fi
mkdir -p "$temp/fixture-bin"
export DEVKIT_GH="$temp/fixture-bin/gh-issues"
cat > "$DEVKIT_GH" <<'SH'
#!/bin/sh
exec devkit-fixture github "$@"
SH
chmod +x "$DEVKIT_GH"
export DEVKIT_FIXTURE_BINARY="$temp/bin/devkit"
export DEVKIT_CODEX="$temp/fixture-bin/codex"
cat > "$DEVKIT_CODEX" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
test "$#" -eq 11
test "$1 $2 $3 $4" = 'exec --sandbox read-only --cd'
test "$5" = "$PWD"
test "$6" = --output-schema
test -s "$7"
test "$8" = -o
test "${10}" = --skip-git-repo-check
test "${11}" = -
prompt="$(cat)"
case "$prompt" in
  *'$ARTIFACTS_DIR'*|*'$PACK_DIR'*) exit 1 ;;
esac
case "$prompt" in
  *"$ARTIFACTS_DIR/judge/diff.patch"*)
    test -s "$ARTIFACTS_DIR/judge/changes.json"
    tree="$(sed -n 's/^  "tree": "\(.*\)",\{0,1\}$/\1/p' "$ARTIFACTS_DIR/judge/target.json")"
    test -n "$tree"
    test "$PWD" = "$tree"
    printf '{"qualities":[]}' > "$9"
    ;;
  *) exit 1 ;;
esac
printf 'Fake Codex transcript\n'
printf 'Fake Codex diagnostics\n' >&2
SH
chmod +x "$DEVKIT_CODEX"
export DEVKIT_CLAUDE="$temp/fixture-bin/claude"
cat > "$DEVKIT_CLAUDE" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
test "$#" -eq 11 -o "$#" -eq 13
test "$1 $2" = '-p --json-schema'
test "$4 $5 $6 $7 $8 $9 ${10} ${11}" = '--allowedTools Read,Grep,Glob --tools Read,Grep,Glob --permission-mode dontAsk --output-format json'
prompt="$(cat)"
case "$prompt" in
  *'$ARTIFACTS_DIR'*|*'$PACK_DIR'*) exit 1 ;;
esac
case "$prompt" in
  *"$ARTIFACTS_DIR/judge/diff.patch"*)
    test -s "$ARTIFACTS_DIR/judge/changes.json"
    tree="$(sed -n 's/^  "tree": "\(.*\)",\{0,1\}$/\1/p' "$ARTIFACTS_DIR/judge/target.json")"
    test -n "$tree"
    test "${12}" = --add-dir
    test "${13}" = "$tree"
    answer='{"qualities":[]}'
    ;;
  *) exit 1 ;;
esac
printf '{"type":"result","subtype":"success","is_error":false,"structured_output":%s}\n' "$answer"
printf 'Fake Claude diagnostics\n' >&2
SH
chmod +x "$DEVKIT_CLAUDE"
cat > "$temp/fixture-bin/devkit" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
export DEVKIT_FIXTURE=1
unset TYPESAFE_API_KEY
export GIT_EDITOR=true
export BASE_BRANCH="devkit-base-$(basename "$PWD")"
devkit-fixture before "$@"
"$DEVKIT_FIXTURE_BINARY" "$@"
devkit-fixture after "$@"
SH
chmod +x "$temp/fixture-bin/devkit"
export PATH="$temp/fixture-bin:$PATH"

status=0
printf '\nStandards ownership\n'
if (cd "$root" && "$temp/bin/devkit" concerns check kit/.devkit/standards); then
  printf 'PASS: devkit concerns check kit/.devkit/standards\n'
else
  status=1
fi
test_provider() {
  local provider="$1"
  shift
  local repo="$temp/repository-$provider"
  local validation validation_status fixtures name
  mkdir -p "$repo"
  git -C "$repo" init -q -b main || return 1
  "$temp/bin/devkit" init --provider "$provider" "$repo" > /dev/null || return 1
  cd "$repo" || return 1
  git add -A || return 1
  git -c user.name=dev-kit -c user.email=dev-kit@example.invalid -c commit.gpgsign=false -c core.hooksPath=/dev/null commit -q --allow-empty -m "Test kit snapshot" || return 1
  printf '\nArchon workflow validation (%s)\n' "$provider"
  validation="$(archon validate workflows 2>&1)"
  validation_status=$?
  printf '%s\n' "$validation"
  if [ "$validation_status" -eq 0 ] && ! printf '%s\n' "$validation" | grep -Eq 'WARN|Warning|[1-9][0-9]* warning'; then
    printf 'PASS: %s archon validate workflows without warnings\n' "$provider"
  else
    printf 'FAIL: %s archon validate workflows\n' "$provider"
    status=1
  fi
  printf '\nFresh devkit init workflow fixtures\n'
  if archon workflow test; then
    printf 'PASS: fresh devkit init archon workflow test\n'
  else
    printf 'FAIL: fresh devkit init archon workflow test\n'
    status=1
  fi
  for fixtures in "$root"/test/fixtures/*; do
    name="$(basename "$fixtures")"
    cp -R "$fixtures/." "$repo/.archon/workflows/dev/$name/fixtures/" || exit 1
  done
  git add -A || exit 1
  git -c user.name=dev-kit -c user.email=dev-kit@example.invalid -c commit.gpgsign=false -c core.hooksPath=/dev/null commit -qm "Add source-only executed fixtures" || exit 1
  printf '\nArchon workflow fixtures\n'
  if [ "$#" -eq 0 ]; then
    if archon workflow test; then
      printf 'PASS: archon workflow test\n'
    else
      printf 'FAIL: archon workflow test\n'
      status=1
    fi
  else
    for name in "$@"; do
      if archon workflow test "$name"; then
        printf 'PASS: archon workflow test %s\n' "$name"
      else
        printf 'FAIL: archon workflow test %s\n' "$name"
        status=1
      fi
    done
  fi
}

for provider in claude codex; do
  if ! test_provider "$provider" "$@"; then
    status=1
  fi
done
printf '\nGo tests\n'
if (cd "$root" && mise exec -- go test ./...); then
  printf 'PASS: go test ./...\n'
else
  printf 'FAIL: go test ./...\n'
  status=1
fi
exit "$status"
