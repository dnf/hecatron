#!/usr/bin/env bash
# Default lint hook: the Makefile's lint target; Free Pascal has no standard linter.
# shellcheck source=_fpc.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_fpc.sh"
if uses_makefile; then native_hook lint; exit; fi
has_pascal_sources || unknown "no Makefile and no .pas or .pp sources"
skip "Free Pascal has no standard linter"
