#!/usr/bin/env bash
# Default lint hook: the formatter's check.
source "$(dirname "$0")/../../_hooks.sh"
[ -f gleam.toml ] || unknown "no gleam.toml"
run gleam format --check .
