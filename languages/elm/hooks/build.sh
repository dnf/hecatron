#!/usr/bin/env bash
# Default build hook: a package's exposed modules, or an application's
# src/Main.elm, compiled without writing output.
source "$(dirname "$0")/../../_hooks.sh"
[ -f elm.json ] || unknown "no elm.json"
if grep -q '"type": *"package"' elm.json; then
    run elm make --output=/dev/null
elif [ -f src/Main.elm ]; then
    run elm make src/Main.elm --output=/dev/null
else
    unknown "an application without src/Main.elm"
fi
