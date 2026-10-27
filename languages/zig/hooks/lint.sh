#!/usr/bin/env bash
# Default lint hook: zig fmt'\''s check, which also checks that each file parses.
source "$(dirname "$0")/../../_hooks.sh"
[ -f build.zig ] || unknown "no build.zig"
checks_format
run zig fmt --check --exclude .zig-cache --exclude zig-cache --exclude zig-out .
