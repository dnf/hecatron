#!/usr/bin/env bash
# Update Lua version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/M\.VERSION = "[^"]*"/M.VERSION = "'"${ver}"'"/' \
    "${ROOT}/age/src/lua/sayableage.lua"
