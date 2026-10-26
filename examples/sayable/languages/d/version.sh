#!/usr/bin/env bash
# Update D version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/enum VERSION = "[^"]*"/enum VERSION = "'"${ver}"'"/' \
    "${ROOT}/age/impl/d/source/sayableage/format_age.d"
_sed_i 's/"version": "[^"]*"/"version": "'"${ver}"'"/' \
    "${ROOT}/age/impl/d/dub.json"
