#!/usr/bin/env bash
# Default format hook: v fmt.
source "$(dirname "$0")/../../_hooks.sh"
[ -f v.mod ] || unknown "no v.mod"
if format_checking; then run v fmt -verify .; else run v fmt -w .; fi
