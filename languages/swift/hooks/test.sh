#!/usr/bin/env bash
# Default test hook: SwiftPM (every test target).
# shellcheck source=languages/_hooks.sh
source "$(dirname "$0")/../../_hooks.sh"
swift_package
run swift test ${swift_package_args[@]+"${swift_package_args[@]}"}
