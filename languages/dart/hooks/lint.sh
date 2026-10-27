#!/usr/bin/env bash
# Default lint hook: the analyzer, failing on infos too (lints report as
# infos), with the package's analysis_options.yaml.
source "$(dirname "$0")/../../_hooks.sh"
[ -f pubspec.yaml ] || unknown "no pubspec.yaml"
dart_deps
run "$(dart_tool)" analyze --fatal-infos
