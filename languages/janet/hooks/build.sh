#!/usr/bin/env bash
# Default build hook: jpm build, into build/.
source "$(dirname "$0")/../../_hooks.sh"
[ -f project.janet ] || unknown "no project.janet"
jpm_ build
