#!/usr/bin/env bash
# Default build hook: the Makefile's default target, else compile every
# unit and program in the directory and src/ with Free Pascal (see _fpc.sh).
# shellcheck source=_fpc.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_fpc.sh"
if uses_makefile; then native_hook build; exit; fi
_sources="$(fpc_sources)"
[ -n "${_sources}" ] || unknown "no Makefile, and no .pas or .pp sources outside the tests"
while IFS= read -r _source; do
    fpc_compile "${_source}"
done <<< "${_sources}"
