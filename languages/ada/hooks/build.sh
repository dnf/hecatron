#!/usr/bin/env bash
# Default build hook: alr build for an Alire crate, else GPRbuild on each
# top-level project file that isn't a test project (see _gpr.sh).
# shellcheck source=_gpr.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_gpr.sh"
if is_alire_crate; then alr_ build; exit; fi
_projects="$(gpr_projects)"
[ -n "${_projects}" ] || unknown "no alire.toml and no .gpr project file"
while IFS= read -r _project; do
    run gprbuild -p -P "${_project}"
done <<< "${_projects}"
