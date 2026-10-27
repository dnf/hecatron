#!/usr/bin/env bash
# Default clean hook: build/, where the build hook puts the packages (see _mojo.sh).
# shellcheck source=_mojo.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_mojo.sh"
[ -n "$(mojo_files)" ] || unknown "no .mojo files"
run rm -rf "${_mojo_build_dir}"
