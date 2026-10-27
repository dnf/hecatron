#!/usr/bin/env bash
# Default clean hook: Wren scripts are interpreted, so leave nothing to clean.
source "$(dirname "$0")/../../_hooks.sh"
skip "Wren scripts leave no build artifacts"
