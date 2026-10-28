#!/usr/bin/env bash
# Default test hook: SwiftPM (every test target). A package that declares
# none (a C library's Package.swift for SwiftPM users) has nothing to test:
# swift test would fail with "no tests found".
# shellcheck source=languages/_hooks.sh
source "$(dirname "$0")/../../_hooks.sh"
swift_package
grep -qs 'testTarget[[:space:]]*(' "${build_dir}"/Package*.swift ||
    skip "the package declares no test targets"
run swift test ${swift_package_args[@]+"${swift_package_args[@]}"}
