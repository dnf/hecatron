#!/usr/bin/env bash
# Run one language's native test suites, for every library or the ones named.
#
# Usage: test.sh <lang> [library ...]     (libraries: age date duration time)
#
# Each language's .hecatron/languages/<lang>/test.sh runs one library's suite.
# It is called from inside <library>/impl/<lang>/ with the library name as $1,
# builds whatever the suite needs, and exits non-zero when a test fails.
# Every library runs even after one fails, so one run reports all of them.
set -euo pipefail

source "$(dirname "$0")/helpers.sh"

_lang="${1:?usage: test.sh <lang> [library ...]}"
shift
_libraries=("$@")
[ "${#_libraries[@]}" -eq 0 ] && _libraries=(age date duration time)

_script="${_ROOT}/.hecatron/languages/${_lang}/test.sh"
if [ ! -f "${_script}" ]; then
    echo "error: ${_script} not found" >&2
    exit 1
fi

# Only Ada gets Alire's toolchain on PATH: it also has gcc/g++, which would
# shadow the system compilers other languages build with.
if [ "${_lang}" = "ada" ]; then _ada_env; fi

_failed=()
for _library in "${_libraries[@]}"; do
    _dir="${_ROOT}/${_library}/impl/${_lang}"
    if [ ! -d "${_dir}" ]; then
        echo "error: ${_dir} not found" >&2
        _failed+=("${_library}")
        continue
    fi
    echo "=== ${_library} (${_lang}) ==="
    if ! (cd "${_dir}" && bash "${_script}" "${_library}"); then
        _failed+=("${_library}")
    fi
done

if [ "${#_failed[@]}" -gt 0 ]; then
    echo "${_lang}: failed: ${_failed[*]}" >&2
    exit 1
fi
echo "${_lang}: all ${#_libraries[@]} libraries passed"
