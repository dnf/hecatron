#!/usr/bin/env bash
# Default test hook: dune runtest (every test stanza and runtest rule).
# shellcheck source=_dune.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_dune.sh"
run dune runtest
