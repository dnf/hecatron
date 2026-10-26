#!/usr/bin/env bash
# Run one library's Mojo test file, through pixi when mojo is not on PATH (as CI does).
set -euo pipefail

_src="${PWD}/src"
_test="${PWD}/tests/test_format_${1}.mojo"
if command -v mojo >/dev/null 2>&1; then
    mojo run -I "${_src}" "${_test}"
else
    pixi run --manifest-path "${PWD}/../../../age/impl/mojo/pixi.toml" mojo run -I "${_src}" "${_test}"
fi
