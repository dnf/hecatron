#!/usr/bin/env bash
# Default test hook: busted, where the directory has specs for it (spec/ or
# a .busted configuration).
source "$(dirname "$0")/../../_hooks.sh"
if [ -f .busted ] || [ -d spec ]; then
    run busted
else
    unknown "no spec/ directory or .busted file for busted"
fi
