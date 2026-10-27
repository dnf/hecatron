#!/usr/bin/env bash
# Default deps hook: nimble install --depsOnly (see deps_nim in languages/_deps.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_deps.sh"
deps_nim
