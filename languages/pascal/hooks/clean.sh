#!/usr/bin/env bash
# Default clean hook: the Makefile's clean target, else build/ (where the
# other hooks put every compiler output, see _fpc.sh).
# shellcheck source=_fpc.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_fpc.sh"
if uses_makefile; then native_hook clean; exit; fi
has_pascal_sources || unknown "no Makefile and no .pas or .pp sources"
run rm -rf "${_fpc_build_dir}"
