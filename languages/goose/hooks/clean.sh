#!/usr/bin/env bash
# Default clean hook: the executables the build hook writes into build/.
# (Goose keeps its intermediates in GOOSE_HOME, shared by every project.)
source "$(dirname "$0")/../../_hooks.sh"
run rm -rf build
