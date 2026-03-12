#!/usr/bin/env bash
# Update Mojo version in source
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/^alias VERSION = "[^"]*"/alias VERSION = "'"${ver}"'"/' \
    "${ROOT}/age/impl/mojo/src/sayable_age/formatter.mojo"
