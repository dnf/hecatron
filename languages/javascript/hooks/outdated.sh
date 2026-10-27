#!/usr/bin/env bash
# Default outdated hook: the package manager's own report (see outdated_node in languages/_outdated.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_outdated.sh"
outdated_node
