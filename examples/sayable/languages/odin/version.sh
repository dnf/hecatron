#!/usr/bin/env bash
# Update Odin version in source
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/VERSION :: "[^"]*"/VERSION :: "'"${ver}"'"/' \
    "${ROOT}/age/impl/odin/sayableage.odin"
