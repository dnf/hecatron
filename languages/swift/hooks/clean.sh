#!/usr/bin/env bash
# Default clean hook: SwiftPM (the build artifacts in .build/, keeping fetched dependencies).
source "$(dirname "$0")/../../_hooks.sh"
[ -f Package.swift ] || unknown "no Package.swift"
run swift package clean
