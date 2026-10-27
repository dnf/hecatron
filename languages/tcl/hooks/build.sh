#!/usr/bin/env bash
# Default build hook: the Makefile of a TEA extension (see native_hook in
# languages/_hooks.sh); Tcl scripts alone have nothing to build.
source "$(dirname "$0")/../../_hooks.sh"
if [ -f Makefile ] || [ -f configure ] || [ -f configure.ac ]; then
    native_hook build
else
    skip "no Makefile, and Tcl scripts need no build"
fi
