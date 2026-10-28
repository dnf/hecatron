#!/usr/bin/env bash
# Default test hook: Cargo's default test targets: the library's, binaries'
# and integration tests, examples built, and doctests. Not --all-targets,
# which drops doctests and runs benchmarks as tests (nightly-only or
# long-running ones fail there).
source "$(dirname "$0")/../../_hooks.sh"
# Cargo finds the package or workspace from any directory inside it.
find_build --above-first Cargo.toml || unknown "no Cargo.toml here, below, or above it in the project"
run cargo test
