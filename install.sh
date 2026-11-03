#!/bin/sh
set -efu

if [ "$#" -gt 1 ] || [ "${1-}" = "--help" ] || [ "${1-}" = "-h" ]; then
    printf '%s\n' 'Usage: /path/to/dev-kit/install.sh [bin-directory]' 'Run from the repository to initialize. The bin directory must be on PATH.'
    [ "$#" -le 1 ] && exit 0
    exit 1
fi

source_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
target_dir=$(pwd -P)
bin_dir=${1:-"$HOME/.local/bin"}
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
