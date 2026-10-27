#!/usr/bin/env bash
# Default build hook: shell scripts aren't built.
source "$(dirname "$0")/../../_hooks.sh"
skip "shell scripts have nothing to build"
