#!/usr/bin/env bash
# Default test hook: elm-test over tests/.
source "$(dirname "$0")/../../_hooks.sh"
[ -f elm.json ] || unknown "no elm.json"
run elm-test
