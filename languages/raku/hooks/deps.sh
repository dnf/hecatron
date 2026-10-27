#!/usr/bin/env bash
# Default deps hook: zef install --deps-only (see deps_raku in languages/_deps.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_deps.sh"
deps_raku
