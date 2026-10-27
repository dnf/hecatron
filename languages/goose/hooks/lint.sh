#!/usr/bin/env bash
# Default lint hook: goose check (parse and typecheck) on each module here
# and each program in test/, tests/ and examples/, failing on any error or
# warning.
# shellcheck source=languages/_hooks.sh
source "$(dirname "$0")/../../_hooks.sh"
find_files -name '*.goose' \( -path './*.goose' -not -path './*/*' -o -path './test/*' -o -path './tests/*' -o -path './examples/*' \)
[ "${#found_files[@]}" -gt 0 ] || unknown "no .goose files"
_failed=0
for _file in "${found_files[@]}"; do
    echo "+ goose check --diagnostics json ${_file}"
    _diagnostics="$(goose check --diagnostics json "${_file}" 2>/dev/null)" || _failed=1
    if [ "${_diagnostics}" != "[]" ]; then
        echo "${_diagnostics}"
        _failed=1
    fi
done
exit "${_failed}"
