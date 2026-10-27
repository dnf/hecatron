#!/usr/bin/env bash
# Default deps hook: Pkg.instantiate (see deps_julia in languages/_deps.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_deps.sh"
deps_julia
