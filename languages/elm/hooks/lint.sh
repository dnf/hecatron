#!/usr/bin/env bash
# Default lint hook: elm-review, where the project configures it (review/elm.json).
source "$(dirname "$0")/../../_hooks.sh"
[ -f elm.json ] || unknown "no elm.json"
[ -f review/elm.json ] || skip "no elm-review configuration (review/elm.json)"
run elm-review
