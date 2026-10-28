#!/usr/bin/env bash
# Default test hook: raco test on every module here (their test submodules,
# honouring info.rkt's test-omit-paths). A repository of several packages
# side by side (deta/, deta-lib/, deta-test/, each with its info.rkt) has
# them installed first, linked to this checkout, with their dependencies:
# their modules require each other's collections.
# shellcheck source=languages/_hooks.sh
source "$(dirname "$0")/../../_hooks.sh"
find_files -name '*.rkt'
[ "${#found_files[@]}" -gt 0 ] || unknown "no .rkt files"
if [ ! -f info.rkt ]; then
    _packages=()
    for _info in */info.rkt; do
        if [ -f "${_info}" ]; then _packages+=("$(dirname "${_info}")"); fi
    done
    if [ "${#_packages[@]}" -gt 0 ]; then
        run raco pkg install --auto --batch --skip-installed --link "${_packages[@]}"
    fi
fi
run raco test .
