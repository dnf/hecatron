# Smoke test for wren: build and run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf 'System.print("hello")\n' > hello.wren
wren_cli hello.wren
