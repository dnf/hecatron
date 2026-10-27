#!/usr/bin/env bash
# Default test hook: SwiftPM (every test target).
source "$(dirname "$0")/../../_hooks.sh"
[ -f Package.swift ] || unknown "no Package.swift"
run swift test
