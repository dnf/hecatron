#!/usr/bin/env bash
# Default build hook: the module, as an executable when it has a main module, else as a shared library (both named after the directory, as V does).
source "$(dirname "$0")/../../_hooks.sh"
[ -f v.mod ] || unknown "no v.mod"
if grep -Eq '^module[[:space:]]+main\b|^fn[[:space:]]+main\(' ./*.v; then
    run v .
else
    run v -shared .
fi
