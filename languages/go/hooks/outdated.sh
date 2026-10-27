#!/usr/bin/env bash
# Default outdated hook: go list -u -m (see outdated_go in languages/_outdated.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_outdated.sh"
outdated_go
