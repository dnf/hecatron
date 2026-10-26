#!/usr/bin/env bash
# Build and run one library's native Ada test suite.
set -euo pipefail

_largs=()
if [ "$(uname -s)" = "Darwin" ]; then
    _sdk="$(xcrun --show-sdk-path 2>/dev/null || true)"
    [ -n "${_sdk}" ] && _largs=(-largs "-L${_sdk}/usr/lib")
fi

gprbuild -P "sayable${1}.gpr" -XBUILD_MODE=test "${_largs[@]+"${_largs[@]}"}"
"./build/test_format_${1}"
