#!/usr/bin/env bash
# Default build hook: a native executable in build/ for each bin/ entry
# point; a library otherwise has nothing to build.
source "$(dirname "$0")/../../_hooks.sh"
[ -f pubspec.yaml ] || unknown "no pubspec.yaml"
[ "$(dart_tool)" = dart ] || skip "a Flutter build needs a target platform"
exists 'bin/*.dart' || skip "no bin/*.dart entry points"
dart_deps
mkdir -p build
for _main in bin/*.dart; do
    run dart compile exe "${_main}" -o "build/$(basename "${_main}" .dart)"
done
