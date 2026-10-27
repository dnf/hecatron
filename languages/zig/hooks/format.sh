#!/usr/bin/env bash
# Default format hook: zig fmt.
source "$(dirname "$0")/../../_hooks.sh"
[ -f build.zig ] || unknown "no build.zig"
_excludes=(--exclude .zig-cache --exclude zig-cache --exclude zig-out)
if format_checking; then run zig fmt --check "${_excludes[@]}" .; else run zig fmt "${_excludes[@]}" .; fi
