#!/usr/bin/env bash
# Default prune hook: the build's caches (.zig-cache/, and zig-cache/ from
# Zig before 0.13), each whole once no build has used it for 7d (prune.py
# zig: a Zig cache can't be trimmed entry by entry). zig-out/ stays.
source "$(dirname "$0")/../../_hooks.sh"
[ -f build.zig ] || unknown "no build.zig"
prune_py zig "${ZIG_LOCAL_CACHE_DIR:-.zig-cache}" zig-cache
if prune_caches; then
    prune_say "Zig's global cache stays: it holds the packages' sources (p/), and can't be trimmed entry by entry"
fi
