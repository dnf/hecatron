#!/usr/bin/env bash
# Default clean hook: fpm's build directory (keeping the dependencies it
# fetched) when the directory has an fpm.toml, else CMake, Meson, Autotools or
# make (see native_hook in languages/_hooks.sh).
source "$(dirname "$0")/../../_hooks.sh"
if [ -f fpm.toml ]; then run fpm clean --skip; else native_hook clean; fi
