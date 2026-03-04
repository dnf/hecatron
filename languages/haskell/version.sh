#!/usr/bin/env bash
# Update Haskell version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
local_pvp="${ver}.0"
_sed_i 's/^version = "[^"]*"/version = "'"${ver}"'"/' \
    "${ROOT}/src/haskell/src/ParentTime/Formatter.hs"
_sed_i 's/^version:       [0-9][^ ]*/version:       '"${local_pvp}"'/' \
    "${ROOT}/src/haskell/parent-time.cabal"
