#!/usr/bin/env bash
# Default clean hook: remove the files `go build` and `go test` leave in the module.
source "$(dirname "$0")/../../_hooks.sh"
# The go command finds the module from any directory inside it.
find_build --above-first go.mod || unknown "no go.mod here, below, or above it in the project"
run go clean ./...
