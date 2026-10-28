#!/usr/bin/env bash
# Default test hook: Bats, on the .bats files under test/ or tests/, else
# those in the directory itself. Files under a fixtures/ directory are test
# data (bats-core's own deliberately broken suites), not tests.
source "$(dirname "$0")/../../_hooks.sh"
_tests=()
for _dir in test tests; do
    [ -d "${_dir}" ] || continue
    while IFS= read -r -d '' _file; do
        _tests+=("${_file#./}")
    done < <(find "${_dir}" -name fixtures -prune -o -name '*.bats' -type f -print0 | sort -z)
done
if [ "${#_tests[@]}" -gt 0 ]; then
    run bats "${_tests[@]}"
elif exists '*.bats'; then
    run bats .
else
    unknown "no .bats files in test/, tests/ or here"
fi
