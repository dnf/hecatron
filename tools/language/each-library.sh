#!/usr/bin/env bash
# Run one of a language's per-library scripts (test.sh, lint.sh, build.sh or clean.sh) for every
# library, or the ones named.
#
# Usage: each-library.sh <test|lint|build|clean> <lang> [library ...]
#        (libraries: age date duration time)
#
# .hecatron/languages/<lang>/<action>.sh handles one library. It is called
# from inside <library>/impl/<lang>/ with the library name as $1, builds or
# installs whatever it needs, and exits non-zero on a failure. Every library
# runs even after one fails, so one run reports all of them.
set -euo pipefail

source "$(dirname "$0")/helpers.sh"

_action="${1:?usage: each-library.sh <test|lint|build|clean> <lang> [library ...]}"
_lang="${2:?usage: each-library.sh <test|lint|build|clean> <lang> [library ...]}"
shift 2
_libraries=("$@")
[ "${#_libraries[@]}" -eq 0 ] && _libraries=(age date duration time)

_script="${_ROOT}/.hecatron/languages/${_lang}/${_action}.sh"
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
    echo "=== ${_action} ${_library} (${_lang}) ==="
    if ! (cd "${_dir}" && bash "${_script}" "${_library}"); then
        _failed+=("${_library}")
    fi
done

if [ "${#_failed[@]}" -gt 0 ]; then
    echo "${_lang}: failed: ${_failed[*]}" >&2
    exit 1
fi
echo "${_lang}: all ${#_libraries[@]} libraries passed"
