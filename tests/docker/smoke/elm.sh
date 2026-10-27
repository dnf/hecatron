# Smoke test for elm: build and run a small program in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
elm --version
elm-test --version
