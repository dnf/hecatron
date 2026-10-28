#!/usr/bin/env bash
# Default test hook: elm-test over tests/. elm-test 0.19.1-revision10 and
# later run only elm-explorations/test 2.x, so a project still on 1.x runs
# on the last elm-test that supports it; and an application pinned to Elm
# 0.19.1 ("elm-version") on that compiler when the installed elm is another
# (elm make refuses a mismatch). Both through npx.
source "$(dirname "$0")/../../_hooks.sh"
[ -f elm.json ] || unknown "no elm.json"
_runner=elm-test
_compiler=()
if grep -qE '"elm-explorations/test"[[:space:]]*:[[:space:]]*"1\.' elm.json; then
    _runner=elm-test@0.19.1-revision9
fi
if grep -qE '"elm-version"[[:space:]]*:[[:space:]]*"0\.19\.1"' elm.json && [ "$(elm --version 2>/dev/null)" != 0.19.1 ]; then
    _compiler=(-p elm@0.19.1-6)
fi
if [ "${_runner}" = elm-test ] && [ "${#_compiler[@]}" -eq 0 ]; then
    run elm-test
else
    run npx --yes "${_compiler[@]+"${_compiler[@]}"}" -p "${_runner}" elm-test
fi
