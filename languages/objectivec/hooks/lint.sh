#!/usr/bin/env bash
# Default lint hook: xcodebuild, GNUstep make, CMake, Meson, Autotools or make, whichever the directory uses (see objc_hook in languages/_hooks.sh).
source "$(dirname "$0")/../../_hooks.sh"
objc_hook lint
