#!/usr/bin/env bash
# Default clean hook: CMake, Meson, Autotools or make, whichever the directory uses (see native_hook in languages/_hooks.sh).
source "$(dirname "$0")/../../_hooks.sh"
native_hook clean
