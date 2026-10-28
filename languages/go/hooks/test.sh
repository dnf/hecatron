#!/usr/bin/env bash
# Default test hook: the module's tests.
source "$(dirname "$0")/../../_hooks.sh"
# The go command finds the module from any directory inside it.
find_build --above-first go.mod || unknown "no go.mod here, below, or above it in the project"
if [ -n "${HECATRON_TEST_REPORTS:-}" ] && [ -n "${HECATRON_BINARY:-}" ]; then
    # Asked for a JUnit report: go test's events saved for it, its usual
    # output in the log (hecatron _gotest prints what go test would).
    echo "+ go test -json ./..."
    go test -json ./... 2>&1 | "${HECATRON_BINARY}" _gotest "${HECATRON_TEST_REPORTS}/go-test.json"
    exit
fi
run go test ./...
