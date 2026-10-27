#!/usr/bin/env bash
# Default build hook: fpm when the directory has an fpm.toml, else CMake,
# Meson, Autotools or make (see native_hook in languages/_hooks.sh).
source "$(dirname "$0")/../../_hooks.sh"
if [ -f fpm.toml ]; then run fpm build; else native_hook build; fi
