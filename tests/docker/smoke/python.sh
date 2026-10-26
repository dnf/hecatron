# Smoke test for python: build and run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
uv init --quiet hello && cd hello && uv run python -c 'print("hello")'
