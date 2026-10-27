#!/usr/bin/env bash
# Default test hook: SWI-Prolog's plunit. Loads the test files in test/ or
# tests/ (.pl or .plt), and the source beside each other .plt file with
# load_test_files/1, then runs every test, failing when any does. A pack's
# prolog/ directory is on the library path, as when it is installed.
# shellcheck source=languages/_hooks.sh
source "$(dirname "$0")/../../_hooks.sh"
_load=()
find_files -name '*.plt' -o -path './test/*.pl' -o -path './tests/*.pl'
for _file in ${found_files[@]+"${found_files[@]}"}; do
    case "${_file}" in
        ./test/* | ./tests/*) _load+=("${_file}") ;;
        *) if [ -f "${_file%.plt}.pl" ]; then _load+=("${_file%.plt}.pl"); else _load+=("${_file}"); fi ;;
    esac
done
[ "${#_load[@]}" -gt 0 ] || unknown "no test files (test/*.pl, *.plt)"
_files=""
for _file in "${_load[@]}"; do
    _files="${_files:+${_files}, }'${_file//\'/\\\'}'"
done
_library=()
[ -d prolog ] && _library=(-p library=prolog)
run swipl ${_library[@]+"${_library[@]}"} -g "consult([${_files}]), load_test_files([]), run_tests" -t halt
