#!/usr/bin/env bash
# Default build hook: gleam build.
source "$(dirname "$0")/../../_hooks.sh"
[ -f gleam.toml ] || unknown "no gleam.toml"
run gleam build
