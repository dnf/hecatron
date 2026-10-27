#!/usr/bin/env bash
# Default test hook: dart test (flutter test for a Flutter package).
source "$(dirname "$0")/../../_hooks.sh"
[ -f pubspec.yaml ] || unknown "no pubspec.yaml"
dart_deps
run "$(dart_tool)" test
