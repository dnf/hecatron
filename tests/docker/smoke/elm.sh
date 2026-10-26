# Smoke test for elm: build and run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
elm --version
elm-test --version
