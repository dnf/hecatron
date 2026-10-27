#!/usr/bin/env bash
# Default clean hook: the fasls ASDF compiled for this directory (see
# asdf_hook in languages/_hooks.sh).
source "$(dirname "$0")/../../_hooks.sh"
asdf_hook clean
