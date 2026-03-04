#!/usr/bin/env bash
# Update Ada version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/Version : constant String := "[^"]*"/Version : constant String := "'"${ver}"'"/' \
    "${ROOT}/sayable-age/src/ada/src/sayableage.ads"
