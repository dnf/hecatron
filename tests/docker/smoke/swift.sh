# Smoke test for swift: build and run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
mkdir hello && cd hello && swift package init --type executable >/dev/null && swift run
