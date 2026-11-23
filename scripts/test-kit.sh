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
repo="$temp/repository"
mkdir -p "$repo"
git -C "$repo" init -q -b main || exit 1
devkit init "$repo" > /dev/null || exit 1
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
cd "$repo" || exit 1
git add -A || exit 1
git -c user.name=dev-kit -c user.email=dev-kit@example.invalid -c commit.gpgsign=false -c core.hooksPath=/dev/null commit -q --allow-empty -m "Test kit snapshot" || exit 1

status=0
printf '\nArchon workflow validation\n'
if archon validate workflows; then
  printf 'PASS: archon validate workflows\n'
else
  printf 'FAIL: archon validate workflows\n'
  status=1
fi
printf '\nStandards ownership\n'
if (cd "$root" && devkit concerns check kit/.devkit/standards); then
  printf 'PASS: devkit concerns check kit/.devkit/standards\n'
else
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
mkdir -p "$temp/fixture-bin"
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
    printf '{"qualities":[]}' > "$9"
    ;;
  *"$ARTIFACTS_DIR/refine/reflect-input.json"*)
    test -s "$ARTIFACTS_DIR/refine/reflect-input.json"
    printf '{"not_reflected":[]}' > "$9"
    ;;
  *) exit 1 ;;
esac
printf 'Fake Codex transcript\n'
printf 'Fake Codex diagnostics\n' >&2
SH
chmod +x "$DEVKIT_CODEX"
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
printf '\nGo tests\n'
if (cd "$root" && mise exec -- go test ./...); then
  printf 'PASS: go test ./...\n'
else
  printf 'FAIL: go test ./...\n'
  status=1
fi
exit "$status"
