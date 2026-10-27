#!/usr/bin/env bash
# Default deps hook: renv's restore (see deps_r in languages/_deps.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_deps.sh"
deps_r
