# Smoke test for lua: build and run a small program in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
lua -e 'print("hello")'
busted --version
luacheck --version
