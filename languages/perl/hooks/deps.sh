#!/usr/bin/env bash
# Default deps hook: Carton's deployment install (see deps_perl in languages/_deps.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_deps.sh"
deps_perl
