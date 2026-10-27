#!/usr/bin/env bash
# Default lint hook: golangci-lint (which runs go vet among others).
source "$(dirname "$0")/../../_hooks.sh"
# The go command finds the module from any directory inside it.
find_build --above-first go.mod || unknown "no go.mod here, below, or above it in the project"
run golangci-lint run ./...
