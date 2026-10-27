#!/usr/bin/env bash
# Default test hook: the Makefile's check or test target, else compile and run
# each test program with Free Pascal (see _fpc.sh).
# shellcheck source=_fpc.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_fpc.sh"
if uses_makefile; then native_hook test; exit; fi
_tests="$(fpc_tests)"
[ -n "${_tests}" ] || unknown "no Makefile, and no test*.pas, *_test.pas or tests/*.pas"
while IFS= read -r _test; do
    _program="$(basename "${_test%.*}")"
    fpc_compile -o"${_fpc_build_dir}/${_program}" "${_test}"
    run "./${_fpc_build_dir}/${_program}"
done <<< "${_tests}"
