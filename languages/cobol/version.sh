#!/usr/bin/env bash
# Update COBOL version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i '/PT-VERSION/s/VALUE "[^"]*"/VALUE "'"${ver}"'"/' \
    "${ROOT}/src/cobol/src/Types.cbl"
