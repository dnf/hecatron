#!/usr/bin/env bash
# Default test hook: Cargo (every target's tests).
source "$(dirname "$0")/../../_hooks.sh"
[ -f Cargo.toml ] || unknown "no Cargo.toml"
run cargo test --all-targets
