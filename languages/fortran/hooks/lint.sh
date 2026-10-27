#!/usr/bin/env bash
# Default lint hook: fpm has no lint command and Fortran no standard linter;
# for the native build systems, see native_hook in languages/_hooks.sh.
source "$(dirname "$0")/../../_hooks.sh"
if [ -f fpm.toml ]; then skip "fpm has no lint command"; else native_hook lint; fi
