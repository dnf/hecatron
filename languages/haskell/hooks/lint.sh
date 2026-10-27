#!/usr/bin/env bash
# Default lint hook: HLint where the project configures it (.hlint.yaml), and
# cabal check (cabal's checks of a package description) for each *.cabal.
# shellcheck source=_haskell.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_haskell.sh"
haskell_project
_linted=0
if [ -f .hlint.yaml ]; then run hlint .; _linted=1; fi
if exists '*.cabal'; then run cabal check; _linted=1; fi
[ "${_linted}" -eq 1 ] || skip "no .hlint.yaml and no *.cabal to check"
