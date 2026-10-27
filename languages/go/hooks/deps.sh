#!/usr/bin/env bash
# Default deps hook: go mod download (see deps_go in languages/_deps.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_deps.sh"
deps_go
