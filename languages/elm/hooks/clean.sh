#!/usr/bin/env bash
# Default clean hook: elm-stuff/, the compiler's per-project build cache
# (packages live in ~/.elm, so they stay).
source "$(dirname "$0")/../../_hooks.sh"
[ -f elm.json ] || unknown "no elm.json"
run rm -rf elm-stuff
