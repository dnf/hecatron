#!/usr/bin/env bash
# Default test hook: the module's tests.
source "$(dirname "$0")/../../_hooks.sh"
[ -f go.mod ] || unknown "no go.mod"
run go test ./...
