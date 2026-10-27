#!/usr/bin/env bash
# Default outdated hook: pub outdated (see outdated_dart in languages/_outdated.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_outdated.sh"
outdated_dart
