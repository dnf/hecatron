#!/usr/bin/env bash
# Default test hook: elm-test over tests/. elm-test 0.19.1-revision10 and
# later run only elm-explorations/test 2.x; a package or application still
# on 1.x runs on the last elm-test that supports it.
source "$(dirname "$0")/../../_hooks.sh"
[ -f elm.json ] || unknown "no elm.json"
if grep -qE '"elm-explorations/test"[[:space:]]*:[[:space:]]*"1\.' elm.json; then
    run npx --yes elm-test@0.19.1-revision9
else
    run elm-test
fi
