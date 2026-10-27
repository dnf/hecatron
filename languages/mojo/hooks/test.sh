#!/usr/bin/env bash
# Default test hook: run each test_*.mojo and *_test.mojo, with the packages
# importable (see _mojo.sh).
# shellcheck source=_mojo.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_mojo.sh"
_tests="$(mojo_tests)"
[ -n "${_tests}" ] || unknown "no test_*.mojo or *_test.mojo"
while IFS= read -r _test; do
    run mojo run -I "$(mojo_source_root)" "${_test}"
done <<< "${_tests}"
