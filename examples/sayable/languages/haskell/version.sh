#!/usr/bin/env bash
# Update Haskell version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
local_pvp="${ver}.0"
_sed_i 's/^version = "[^"]*"/version = "'"${ver}"'"/' \
    "${ROOT}/age/impl/haskell/src/SayableAge/Formatter.hs"
_sed_i 's/^version:       [0-9][^ ]*/version:       '"${local_pvp}"'/' \
    "${ROOT}/age/impl/haskell/sayable-age.cabal"
