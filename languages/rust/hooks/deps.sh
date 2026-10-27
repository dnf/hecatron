#!/usr/bin/env bash
# Default deps hook: cargo fetch, --locked with a Cargo.lock (see deps_rust in languages/_deps.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_deps.sh"
deps_rust
