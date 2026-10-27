#!/usr/bin/env bash
# Default outdated hook: Cargo's dry-run update (see outdated_rust in languages/_outdated.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_outdated.sh"
outdated_rust
