#!/usr/bin/env bash
# Default outdated hook: cabal outdated (see outdated_haskell in languages/_outdated.sh).
# shellcheck source=_haskell.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_haskell.sh"
source "$(dirname "$0")/../../_outdated.sh"
outdated_haskell
