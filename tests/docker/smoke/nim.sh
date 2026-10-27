# Smoke test for nim: build and run a small program in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf 'echo "hello"\n' > hello.nim
nim compile --hints:off --run hello.nim
