#!/usr/bin/env bash
# Default clean hook: alr clean for an Alire crate, else gprclean on every
# project file, test projects too (see _gpr.sh).
# shellcheck source=_gpr.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_gpr.sh"
if is_alire_crate; then alr_ clean; exit; fi
_projects="$(gpr_projects; gpr_test_projects)"
[ -n "${_projects}" ] || unknown "no alire.toml and no .gpr project file"
while IFS= read -r _project; do
    run gprclean -r -P "${_project}"
done <<< "${_projects}"
