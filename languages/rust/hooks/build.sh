#!/usr/bin/env bash
# Default build hook: Cargo (every target).
source "$(dirname "$0")/../../_hooks.sh"
[ -f Cargo.toml ] || unknown "no Cargo.toml"
run cargo build --all-targets
