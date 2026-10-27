#!/usr/bin/env bash
# Default build hook: compile every package.
source "$(dirname "$0")/../../_hooks.sh"
[ -f go.mod ] || unknown "no go.mod"
run go build ./...
