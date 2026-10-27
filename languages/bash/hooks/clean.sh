#!/usr/bin/env bash
# Default clean hook: shell scripts aren't built, so leave nothing to clean.
source "$(dirname "$0")/../../_hooks.sh"
skip "shell scripts leave no build artifacts"
