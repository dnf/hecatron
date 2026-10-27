#!/usr/bin/env bash
# Default build hook: load the Tonel repository's baseline into a fresh image in build/ (see pharo_hook in languages/_hooks.sh).
source "$(dirname "$0")/../../_hooks.sh"
pharo_hook build
