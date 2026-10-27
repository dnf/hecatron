#!/usr/bin/env bash
# Default build hook: the build'\''s install step (into zig-out/).
source "$(dirname "$0")/../../_hooks.sh"
[ -f build.zig ] || unknown "no build.zig"
run zig build
