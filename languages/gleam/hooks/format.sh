#!/usr/bin/env bash
# Default format hook: gleam format.
source "$(dirname "$0")/../../_hooks.sh"
[ -f gleam.toml ] || unknown "no gleam.toml"
if format_checking; then run gleam format --check .; else run gleam format .; fi
