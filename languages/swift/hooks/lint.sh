#!/usr/bin/env bash
# Default lint hook: swift-format, with the package'\''s own .swift-format configuration if any, on the package'\''s sources.
source "$(dirname "$0")/../../_hooks.sh"
[ -f Package.swift ] || unknown "no Package.swift"
_sources=(Package.swift)
for _dir in Sources Tests Plugins; do
    [ -d "${_dir}" ] && _sources+=("${_dir}")
done
run swift format lint --strict --recursive "${_sources[@]}"
