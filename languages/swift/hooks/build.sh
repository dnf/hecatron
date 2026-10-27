#!/usr/bin/env bash
# Default build hook: SwiftPM (every target).
# shellcheck source=languages/_hooks.sh
source "$(dirname "$0")/../../_hooks.sh"
swift_package
run swift build --build-tests ${swift_package_args[@]+"${swift_package_args[@]}"}
