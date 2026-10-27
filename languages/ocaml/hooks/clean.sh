#!/usr/bin/env bash
# Default clean hook: dune clean (_build/).
# shellcheck source=_dune.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_dune.sh"
run dune clean
