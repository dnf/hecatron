#!/usr/bin/env bash
# Run the library's raco test suite.
set -euo pipefail
case "$1" in
    age) raco test test/ ;;
    *) raco test "test-format-$1.rkt" ;;
esac
