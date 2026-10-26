#!/usr/bin/env bash
# Type-check and compile one library's Nim formatter module (it has no main, so nothing is linked).
set -euo pipefail

nim check --path:src "src/sayable_${1}/formatter.nim"
