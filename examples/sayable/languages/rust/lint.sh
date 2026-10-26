#!/usr/bin/env bash
# Lint one Rust library and its tests, warnings as errors ($1: age, date, duration or time).
set -euo pipefail
cargo clippy --all-targets -- -D warnings
