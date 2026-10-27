#!/usr/bin/env bash
# Default clean hook: PowerShell scripts and modules are interpreted, so
# leave nothing to clean.
source "$(dirname "$0")/../../_hooks.sh"
skip "PowerShell scripts leave no build artifacts"
