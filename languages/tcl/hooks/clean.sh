#!/usr/bin/env bash
# Default clean hook: the Makefile of a TEA extension (see native_hook in
# languages/_hooks.sh); Tcl scripts alone have nothing to clean.
source "$(dirname "$0")/../../_hooks.sh"
if [ -f Makefile ] || [ -f configure ] || [ -f configure.ac ]; then
    native_hook clean
else
    skip "no Makefile, and Tcl scripts need no build"
fi
