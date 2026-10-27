#!/usr/bin/env bash
# Default deps hook: jpm deps (see deps_janet in languages/_deps.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_deps.sh"
deps_janet
