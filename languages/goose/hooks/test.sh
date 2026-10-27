#!/usr/bin/env bash
# Default test hook: run each test program (the root files in test/ or
# tests/) with goose, failing when any fails, as a failed assert does.
# shellcheck source=languages/_hooks.sh
source "$(dirname "$0")/../../_hooks.sh"
find_files -name '*.goose' \( -path './test/*' -o -path './tests/*' \)
[ "${#found_files[@]}" -gt 0 ] || unknown "no test programs in test/ or tests/"
_failed=0
for _file in "${found_files[@]}"; do
    run goose run "${_file}" || _failed=1
done
exit "${_failed}"
