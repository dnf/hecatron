#!/usr/bin/env bash
# Default lint hook: Clippy with warnings as errors, and rustfmt's check.
source "$(dirname "$0")/../../_hooks.sh"
[ -f Cargo.toml ] || unknown "no Cargo.toml"
run cargo clippy --all-targets -- -D warnings
run cargo fmt --all -- --check
