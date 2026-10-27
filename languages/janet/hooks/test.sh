#!/usr/bin/env bash
# Default test hook: jpm's test (every .janet file in test/), in the project's
# local jpm_tree when it has one.
source "$(dirname "$0")/../../_hooks.sh"
[ -f project.janet ] || unknown "no project.janet"
jpm_ test
