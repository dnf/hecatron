#!/usr/bin/env bash
# Package one library's Mojo sources, through pixi when mojo is not on PATH (as the test does).
set -euo pipefail

mkdir -p build
if command -v mojo >/dev/null 2>&1; then
    mojo package "src/sayable_${1}" -o "build/sayable_${1}.mojopkg"
else
    pixi run --manifest-path "${PWD}/../../../age/impl/mojo/pixi.toml" mojo package "src/sayable_${1}" -o "build/sayable_${1}.mojopkg"
fi
