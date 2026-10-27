#!/usr/bin/env bash
# Default clean hook: Stack's .stack-work or cabal's dist-newstyle, keeping the
# dependencies each has built into its own store (see _haskell.sh).
# shellcheck source=_haskell.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_haskell.sh"
haskell_project
if is_stack; then run stack clean; else run cabal clean; fi
