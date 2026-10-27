#!/usr/bin/env bash
# Default build hook: Cargo (every target).
source "$(dirname "$0")/../../_hooks.sh"
# Cargo finds the package or workspace from any directory inside it.
find_build --above-first Cargo.toml || unknown "no Cargo.toml here, below, or above it in the project"
run cargo build --all-targets
