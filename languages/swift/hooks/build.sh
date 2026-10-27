#!/usr/bin/env bash
# Default build hook: SwiftPM (every target).
source "$(dirname "$0")/../../_hooks.sh"
[ -f Package.swift ] || unknown "no Package.swift"
run swift build --build-tests
