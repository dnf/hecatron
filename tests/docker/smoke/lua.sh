# Smoke test for lua: build and run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
lua -e 'print("hello")'
busted --version
luacheck --version
