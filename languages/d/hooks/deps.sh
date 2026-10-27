#!/usr/bin/env bash
# Default deps hook: dub upgrade --missing-only (see deps_d in languages/_deps.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_deps.sh"
deps_d
