#!/usr/bin/env bash
# Default deps hook: zig build --fetch (see deps_zig in languages/_deps.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_deps.sh"
deps_zig
