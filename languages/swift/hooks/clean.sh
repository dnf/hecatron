#!/usr/bin/env bash
# Default clean hook: SwiftPM (the build artifacts in .build/, keeping fetched dependencies).
# shellcheck source=languages/_hooks.sh
source "$(dirname "$0")/../../_hooks.sh"
swift_package
run swift package ${swift_package_args[@]+"${swift_package_args[@]}"} clean
