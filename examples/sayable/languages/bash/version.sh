#!/usr/bin/env bash
# Update Bash version in source
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/SAYABLEAGE_VERSION="[^"]*"/SAYABLEAGE_VERSION="'"${ver}"'"/' \
    "${ROOT}/age/impl/bash/sayableage.sh"
