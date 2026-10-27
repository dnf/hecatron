#!/usr/bin/env bash
# Default test hook: every *_test.v in the module.
source "$(dirname "$0")/../../_hooks.sh"
[ -f v.mod ] || unknown "no v.mod"
run v test .
