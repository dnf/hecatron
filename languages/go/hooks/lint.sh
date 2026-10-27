#!/usr/bin/env bash
# Default lint hook: golangci-lint (which runs go vet among others).
source "$(dirname "$0")/../../_hooks.sh"
[ -f go.mod ] || unknown "no go.mod"
run golangci-lint run ./...
