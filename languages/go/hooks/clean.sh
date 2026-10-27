#!/usr/bin/env bash
# Default clean hook: remove the files `go build` and `go test` leave in the module.
source "$(dirname "$0")/../../_hooks.sh"
[ -f go.mod ] || unknown "no go.mod"
run go clean ./...
