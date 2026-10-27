#!/usr/bin/env bash
# Default build hook: build each program in test/, tests/ and examples/ into
# an executable in build/ with the machine's C compiler (a library builds
# as part of the programs that use it).
# shellcheck source=languages/_hooks.sh
source "$(dirname "$0")/../../_hooks.sh"
find_files -name '*.goose' \( -path './test/*' -o -path './tests/*' -o -path './examples/*' \)
[ "${#found_files[@]}" -gt 0 ] || skip "no programs in test/, tests/ or examples/"
for _file in "${found_files[@]}"; do
    _name="$(basename "${_file}" .goose)"
    run goose build "${_file}" -o "build/${_name}"
done
