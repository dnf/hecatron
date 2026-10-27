#!/usr/bin/env bash
# Default test hook: every test suite, through Stack or cabal (see _haskell.sh).
# shellcheck source=_haskell.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_haskell.sh"
haskell_project
if is_stack; then run stack test; else run cabal test all --test-show-details=direct; fi
