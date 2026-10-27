#!/usr/bin/env bash
# Default test hook: Wren has no test runner, so run each test script with
# wren_cli (those in test/ or tests/, and *_test.wren or test_*.wren files),
# failing when any fails, as a script does on a runtime error or
# Fiber.abort.
# shellcheck source=languages/_hooks.sh
source "$(dirname "$0")/../../_hooks.sh"
find_files -name '*.wren' \( -path './test/*' -o -path './tests/*' -o -name '*_test.wren' -o -name 'test_*.wren' \)
[ "${#found_files[@]}" -gt 0 ] || unknown "no test scripts (test/*.wren, *_test.wren)"
_failed=0
for _file in "${found_files[@]}"; do
    run wren_cli "${_file}" || _failed=1
done
exit "${_failed}"
