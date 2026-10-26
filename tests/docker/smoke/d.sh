# Smoke test for d: build and run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
dub init --non-interactive --format=json hello >/dev/null
cd hello && dub run --quiet
