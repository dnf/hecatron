#!/usr/bin/env bash
# Update V version in source
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/version = "[^"]*"/version = "'"${ver}"'"/' \
    "${ROOT}/age/impl/v/v.mod"
