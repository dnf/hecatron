#!/usr/bin/env bash
# Default test hook: dart test (flutter test for a Flutter package), which
# runs test/, where package:test and flutter_test look for tests. Where this
# directory isn't a package with tests (a pub workspace's root, a repository
# whose packages are in pkgs/ or runtime/Dart), the packages below that have
# a test/ directory, each.
source "$(dirname "$0")/../../_hooks.sh"
if [ ! -f pubspec.yaml ] || [ ! -d test ]; then
    _packages=()
    while IFS= read -r _dir; do
        if [ -d "${_dir}/test" ]; then _packages+=("${_dir}"); fi
    done < <(dirs_below_with pubspec.yaml)
    [ "${#_packages[@]}" -eq 0 ] || run_hook_in "${_packages[@]}"
    [ -f pubspec.yaml ] || unknown "no pubspec.yaml here or in the directories below"
    unknown "no test/ directory here or in the packages below"
fi
dart_deps
run "$(dart_tool)" test
