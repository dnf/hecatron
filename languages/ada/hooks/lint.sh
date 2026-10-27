#!/usr/bin/env bash
# Default lint hook: Ada has no standard linter (GNAT's style checks are
# compiler switches, which a project sets in its .gpr).
# shellcheck source=_gpr.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_gpr.sh"
is_alire_crate || [ -n "$(gpr_projects)$(gpr_test_projects)" ] || unknown "no alire.toml and no .gpr project file"
skip "Ada has no standard linter"
