#!/usr/bin/env bash
# Default build hook: dune build (the default target: every library and executable).
# shellcheck source=_dune.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_dune.sh"
run dune build
