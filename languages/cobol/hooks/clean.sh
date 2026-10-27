#!/usr/bin/env bash
# Default clean hook: CMake or make, whichever the directory uses (see native_hook in languages/_hooks.sh); GnuCOBOL has no build tool of its own.
source "$(dirname "$0")/../../_hooks.sh"
native_hook clean
