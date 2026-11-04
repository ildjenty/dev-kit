#!/bin/sh
set -efu

usage() {
    printf '%s\n' 'Usage: /path/to/dev-kit/install.sh [--bin DIR] [REPO]' \
        'Builds devkit into DIR (default: $HOME/.local/bin, which must be on PATH)' \
        'and runs devkit init in REPO (default: the current directory).'
}

bin_dir="$HOME/.local/bin"
repo=
while [ "$#" -gt 0 ]; do
    case $1 in
        -h|--help) usage; exit 0 ;;
        --bin)
            [ "$#" -ge 2 ] || { usage >&2; exit 1; }
            bin_dir=$2; shift 2 ;;
        --bin=*) bin_dir=${1#--bin=}; shift ;;
        -*) usage >&2; exit 1 ;;
        *)
            [ -z "$repo" ] || { usage >&2; exit 1; }
            repo=$1; shift ;;
    esac
done

source_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
if [ -n "$repo" ]; then
    [ -d "$repo" ] || { printf 'Repository directory not found: %s\n' "$repo" >&2; exit 1; }
    target_dir=$(CDPATH= cd -- "$repo" && pwd -P)
else
    target_dir=$(pwd -P)
fi
if ! git -C "$target_dir" rev-parse --show-toplevel >/dev/null 2>&1; then
    printf 'Not a git repository: %s\n' "$target_dir" >&2
    exit 1
fi
mkdir -p -- "$bin_dir"
bin_dir=$(CDPATH= cd -- "$bin_dir" && pwd -P)
on_path=false
remaining_path=$PATH
while :; do
    entry=${remaining_path%%:*}
    [ -n "$entry" ] || entry=.
    if [ -d "$entry" ]; then
        entry=$(CDPATH= cd -- "$entry" && pwd -P)
        if [ "$entry" = "$bin_dir" ]; then
            on_path=true
            break
        fi
    fi
    case $remaining_path in
        *:*) remaining_path=${remaining_path#*:} ;;
        *) break ;;
    esac
done
if [ "$on_path" != true ]; then
    printf 'Add %s to PATH, then run this script again.\n' "$bin_dir" >&2
    exit 1
fi

build_dir=$(mktemp -d "${TMPDIR:-/tmp}/devkit-install.XXXXXXXX")
install_file=
cleanup() {
    if [ -n "$install_file" ] && [ -f "$install_file" ]; then
        rm -- "$install_file"
    fi
    if [ -f "$build_dir/devkit" ]; then
        rm -- "$build_dir/devkit"
    fi
    rmdir -- "$build_dir"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
(cd -- "$source_dir" && go build -o "$build_dir/devkit" ./cmd/devkit)
install_file=$(mktemp "$bin_dir/.devkit-install.XXXXXXXX")
cp -- "$build_dir/devkit" "$install_file"
chmod +x "$install_file"
mv -f -- "$install_file" "$bin_dir/devkit"
printf 'Installed %s\n' "$bin_dir/devkit"
cd -- "$target_dir"
"$bin_dir/devkit" init
