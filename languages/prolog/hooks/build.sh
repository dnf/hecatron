#!/usr/bin/env bash
# Default build hook: Prolog sources are loaded, not built.
source "$(dirname "$0")/../../_hooks.sh"
skip "Prolog sources need no build"
