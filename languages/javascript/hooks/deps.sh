#!/usr/bin/env bash
# Default deps hook: the package manager's install, frozen to its lock file (see deps_node in languages/_deps.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_deps.sh"
deps_node
