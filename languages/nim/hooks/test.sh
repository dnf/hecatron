#!/usr/bin/env bash
# Default test hook: nimble (tests/t*.nim, or the package's own test task).
source "$(dirname "$0")/../../_hooks.sh"
exists '*.nimble' || unknown "no .nimble file"
run nimble test -y
