#!/usr/bin/env bash
# Update Wren version in source
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/static version { "[^"]*" }/static version { "'"${ver}"'" }/' \
    "${ROOT}/age/impl/wren/sayableage.wren"
