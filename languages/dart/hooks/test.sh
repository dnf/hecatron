#!/usr/bin/env bash
# Default test hook: dart test (flutter test for a Flutter package), which
# runs test/, where package:test and flutter_test look for tests.
source "$(dirname "$0")/../../_hooks.sh"
[ -f pubspec.yaml ] || unknown "no pubspec.yaml"
[ -d test ] || unknown "no test/ directory"
dart_deps
run "$(dart_tool)" test
