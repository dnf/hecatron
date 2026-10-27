# Smoke test for julia: build and run a small program in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
julia -e 'println("hello")'
