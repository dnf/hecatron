#!/usr/bin/env bash
# Default test hook: load the Tonel repository's baseline into a fresh image in build/ and run its packages' tests (see pharo_hook in languages/_hooks.sh).
source "$(dirname "$0")/../../_hooks.sh"
pharo_hook test
