#!/usr/bin/env bash
# Default clean hook: quick-load files (.qlf) compiled from the sources.
# shellcheck source=languages/_hooks.sh
source "$(dirname "$0")/../../_hooks.sh"
find_files -name '*.qlf'
[ "${#found_files[@]}" -gt 0 ] || skip "no .qlf files"
run rm -f "${found_files[@]}"
