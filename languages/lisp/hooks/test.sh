#!/usr/bin/env bash
# Default test hook: ASDF's test-system on each system here (see asdf_hook
# in languages/_hooks.sh).
source "$(dirname "$0")/../../_hooks.sh"
asdf_hook test
