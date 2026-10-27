# Smoke test for rust: build and run a small program in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
cargo new --quiet hello && cd hello && cargo run --quiet && cargo clippy --quiet -- -D warnings
