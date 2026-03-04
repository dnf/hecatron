#!/usr/bin/env bash
# Update Haskell package metadata
source "$(dirname "$0")/../_common.sh"

_sed_i "s/^license:       .*/license:       ${PT_LICENSE}/" \
    "${ROOT}/src/haskell/parent-time.cabal"
