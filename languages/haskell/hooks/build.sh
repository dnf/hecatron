#!/usr/bin/env bash
# Default build hook: every package, through Stack or cabal (see _haskell.sh).
# shellcheck source=_haskell.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_haskell.sh"
haskell_project
if is_stack; then run stack build; else run cabal build all; fi
