#!/usr/bin/env bash
# Default test hook: Bats, on the .bats files under test/ or tests/, else
# those in the directory itself.
source "$(dirname "$0")/../../_hooks.sh"
_suites=()
for _dir in test tests; do
    if [ -d "${_dir}" ] && [ -n "$(find "${_dir}" -name '*.bats' -print -quit)" ]; then
        _suites+=("${_dir}")
    fi
done
if [ "${#_suites[@]}" -gt 0 ]; then
    run bats -r "${_suites[@]}"
elif exists '*.bats'; then
    run bats .
else
    unknown "no .bats files in test/, tests/ or here"
fi
