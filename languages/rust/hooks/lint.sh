#!/usr/bin/env bash
# Default lint hook: Clippy with warnings as errors, and rustfmt's check.
source "$(dirname "$0")/../../_hooks.sh"
# Cargo finds the package or workspace from any directory inside it.
find_build --above-first Cargo.toml || unknown "no Cargo.toml here, below, or above it in the project"
run cargo clippy --all-targets -- -D warnings
checks_format
run cargo fmt --all -- --check
