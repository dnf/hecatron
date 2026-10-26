#!/usr/bin/env bash
# Run the plunit suite for one library.
set -euo pipefail

case "$1" in
    age) _test="test_format_age" ;;
    *) _test="test_sayable${1}" ;;
esac
swipl -g "consult('${_test}.pl'), run_tests, halt(0)" -t "halt(1)"
