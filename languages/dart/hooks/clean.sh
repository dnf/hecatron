#!/usr/bin/env bash
# Default clean hook: build/ (the build hook's executables, Flutter's
# output); .dart_tool/ stays, since it records the fetched packages.
source "$(dirname "$0")/../../_hooks.sh"
[ -f pubspec.yaml ] || unknown "no pubspec.yaml"
run rm -rf build
