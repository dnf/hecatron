#!/usr/bin/env bash
# Default lint hook: swift-format, with the package'\''s own .swift-format configuration if any, on the package'\''s sources.
# shellcheck source=languages/_hooks.sh
source "$(dirname "$0")/../../_hooks.sh"
swift_package
# A package above builds more than this directory: lint only this directory's sources.
_sources=(.)
[ "${build_dir}" != . ] || _sources=(Package.swift)
for _dir in Sources Tests Plugins; do
    [ "${build_dir}" = . ] && [ -d "${_dir}" ] && _sources+=("${_dir}")
done
checks_format
run swift format lint --strict --recursive "${_sources[@]}"
