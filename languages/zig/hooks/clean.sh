#!/usr/bin/env bash
# Default clean hook: the build's outputs and cache (zig-out/, .zig-cache/,
# and zig-cache/ from Zig before 0.13).
source "$(dirname "$0")/../../_hooks.sh"
[ -f build.zig ] || unknown "no build.zig"
run rm -rf zig-out .zig-cache zig-cache
