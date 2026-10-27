# Smoke test for odin: build and run a small program in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
mkdir hello
printf 'package main\n\nimport "core:fmt"\n\nmain :: proc() {\n\tfmt.println("hello")\n}\n' > hello/main.odin
odin run hello
