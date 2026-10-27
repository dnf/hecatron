#!/usr/bin/env bash
# Default build hook: ASDF compiles and loads each system here (see asdf_hook
# in languages/_hooks.sh).
source "$(dirname "$0")/../../_hooks.sh"
asdf_hook build
