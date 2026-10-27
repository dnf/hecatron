#!/usr/bin/env bash
# The latest release of each PHP release line, newest first, as
# "version<TAB>flags" (see endoflife_releases in _common.sh).
source "$(dirname "$0")/../_common.sh"

endoflife_releases php
