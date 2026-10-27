#!/usr/bin/env bash
# Default outdated hook: uv tree --outdated (see outdated_python in languages/_outdated.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_outdated.sh"
outdated_python
