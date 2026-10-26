#!/usr/bin/env bash
# Remove one library's compiled Racket bytecode.
set -euo pipefail

find . -type d -name compiled -exec rm -rf {} + 2>/dev/null || true
