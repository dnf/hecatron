#!/usr/bin/env bash
# Default test hook: tcltest on the .test files in tests/ (or test/), failing
# when any test does; else the Makefile's check or test target of a TEA
# extension (see native_hook in languages/_hooks.sh).
source "$(dirname "$0")/../../_hooks.sh"
for _dir in tests test; do
    if exists "${_dir}/*.test"; then
        run tclsh "$(dirname "$0")/tcltest.tcl" "${_dir}"
        exit
    fi
done
if [ -f Makefile ] || [ -f configure ] || [ -f configure.ac ]; then
    native_hook test
else
    unknown "no .test files in tests/ or test/, and no Makefile"
fi
