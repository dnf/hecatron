#!/usr/bin/env bash
# Check that swift runs: a toolchain unpacked without its shared libraries is
# on PATH but fails to start (see install.sh). The compiler driver and the
# package manager link different libraries, so try both.
source "$(dirname "$0")/../_common.sh"

if has swift; then
    for _command in "swift --version" "swift package --version"; do
        if ! ${_command} >/dev/null 2>&1; then
            problem "${_command} fails: $(${_command} 2>&1 | head -1)"
        fi
    done
fi
doctor_done
