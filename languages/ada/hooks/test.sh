#!/usr/bin/env bash
# Default test hook: alr test for an Alire crate, else build each GPRbuild test
# project (tests/*.gpr, test/*.gpr, *test*.gpr) and run its mains (see _gpr.sh).
# shellcheck source=_gpr.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_gpr.sh"
# alr test keeps the tests' output in a log, which a failure should show.
_alr_test_log="alire/alr_test_local.log"
if is_alire_crate; then
    alr_ test || { cat "${_alr_test_log}" >&2 2>/dev/null; exit 1; }
    exit
fi
_projects="$(gpr_test_projects)"
[ -n "${_projects}" ] || unknown "no alire.toml, and no tests/*.gpr, test/*.gpr or *test*.gpr"
while IFS= read -r _project; do
    run gprbuild -p -P "${_project}"
    _mains="$(gpr_mains "${_project}")"
    [ -n "${_mains}" ] || unknown "${_project} builds no main to run"
    while IFS= read -r _main; do
        run "${_main}"
    done <<< "${_mains}"
done <<< "${_projects}"
