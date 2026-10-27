#!/usr/bin/env bash
# Default test hook: the package's @(test) procedures.
source "$(dirname "$0")/../../_hooks.sh"
exists '*.odin' || unknown "no .odin files"
run odin test .
