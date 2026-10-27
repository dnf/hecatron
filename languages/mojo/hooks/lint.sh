#!/usr/bin/env bash
# Default lint hook: fail where mojo format would change a file (see
# mojo_format_check in _mojo.sh).
# shellcheck source=_mojo.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_mojo.sh"
checks_format
mojo_format_check
