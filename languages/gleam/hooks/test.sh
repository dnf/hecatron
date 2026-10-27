#!/usr/bin/env bash
# Default test hook: gleam test.
source "$(dirname "$0")/../../_hooks.sh"
[ -f gleam.toml ] || unknown "no gleam.toml"
run gleam test
