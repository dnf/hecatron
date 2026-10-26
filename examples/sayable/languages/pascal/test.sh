#!/usr/bin/env bash
# Compile and run the Free Pascal test program for one library.
set -euo pipefail

case "$1" in
    age) _test="test_format_age" ;;
    *) _test="test_sayable${1}" ;;
esac
mkdir -p build
fpc -FEbuild -FUbuild -Fulocales -o"build/${_test}" "${_test}.pas"
"./build/${_test}"
