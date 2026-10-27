#!/usr/bin/env bash
# Default lint hook: Pharo has no standard command-line linter (see pharo_hook in languages/_hooks.sh).
source "$(dirname "$0")/../../_hooks.sh"
pharo_hook lint
