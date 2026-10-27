#!/usr/bin/env bash
# Default test hook: the build'\''s test step.
source "$(dirname "$0")/../../_hooks.sh"
[ -f build.zig ] || unknown "no build.zig"
run zig build test --summary all
