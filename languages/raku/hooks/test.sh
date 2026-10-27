#!/usr/bin/env bash
# Default test hook: zef test for a distribution (META6.json), else prove
# running each test file in t/ with raku -Ilib.
source "$(dirname "$0")/../../_hooks.sh"
if [ -f META6.json ]; then
    run zef test .
elif [ -d t ]; then
    run prove -r -e 'raku -Ilib' --ext .rakutest --ext .t --ext .t6 t
else
    unknown "no META6.json or t/ directory"
fi
