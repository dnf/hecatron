#!/usr/bin/env bash
# Default clean hook: the compiled/ directories raco make and raco test leave.
source "$(dirname "$0")/../../_hooks.sh"
run find . -type d -name compiled -prune -exec rm -rf {} +
