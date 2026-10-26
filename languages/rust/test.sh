#!/usr/bin/env bash
# Run one library's cargo test suite (the crate in the current directory).
set -euo pipefail

cargo test
