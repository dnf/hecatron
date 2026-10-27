#!/usr/bin/env bash
# Default deps hook: the dependencies built through Stack or cabal (see deps_haskell in
# languages/_deps.sh).
# shellcheck source=_haskell.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_haskell.sh"
source "$(dirname "$0")/../../_deps.sh"
deps_haskell
