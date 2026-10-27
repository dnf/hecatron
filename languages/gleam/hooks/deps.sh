#!/usr/bin/env bash
# Default deps hook: gleam deps download (see deps_gleam in languages/_deps.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_deps.sh"
deps_gleam
