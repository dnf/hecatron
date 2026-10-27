#!/usr/bin/env bash
# Default clean hook: remove build/, the image the other hooks load code into (see pharo_hook in languages/_hooks.sh).
source "$(dirname "$0")/../../_hooks.sh"
pharo_hook clean
