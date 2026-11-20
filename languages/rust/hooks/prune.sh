#!/usr/bin/env bash
# Default prune hook: Cargo's target directory, keeping the builds cargo
# reuses (prune.py cargo: each package and target's builds within 6h of its
# newest). Cargo 1.88 and later trim ~/.cargo's caches themselves.
source "$(dirname "$0")/../../_hooks.sh"
# Cargo finds the package or workspace from any directory inside it.
find_build --above-first Cargo.toml || unknown "no Cargo.toml here, below, or above it in the project"
prune_py cargo "$(cargo_target_dir)"
if prune_caches; then
    prune_say "cargo trims its registry and git caches in ~/.cargo itself (unused for 3 months; [cache] auto-clean-frequency in its config)"
fi
