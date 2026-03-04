#!/usr/bin/env bash
# Update Haskell package metadata
source "$(dirname "$0")/../_common.sh"

_sed_i "s/^license:       .*/license:       ${PT_LICENSE}/" \
    "${ROOT}/sayable-age/src/haskell/sayable-age.cabal"
