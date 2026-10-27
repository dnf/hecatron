#!/usr/bin/env bash
# Default clean hook: jpm clean, which removes build/.
source "$(dirname "$0")/../../_hooks.sh"
[ -f project.janet ] || unknown "no project.janet"
jpm_ clean
