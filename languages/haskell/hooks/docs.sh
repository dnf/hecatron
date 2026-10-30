#!/usr/bin/env bash
# Default docs hook: Haddock through Stack or cabal (see docs_haskell in languages/_docs.sh).
# shellcheck source=_haskell.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_haskell.sh"
source "$(dirname "$0")/../../_docs.sh"
docs_haskell
