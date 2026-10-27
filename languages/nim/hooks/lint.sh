#!/usr/bin/env bash
# Default lint hook: Nim has no standard linter (the build hook runs nim check).
source "$(dirname "$0")/../../_hooks.sh"
exists '*.nimble' || unknown "no .nimble file"
skip "Nim has no standard linter"
