#!/usr/bin/env bash
# Default clean hook: Cargo (target/).
source "$(dirname "$0")/../../_hooks.sh"
[ -f Cargo.toml ] || unknown "no Cargo.toml"
run cargo clean
