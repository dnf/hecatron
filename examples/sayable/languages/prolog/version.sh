#!/usr/bin/env bash
# Update Prolog version in source
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i "s/version('[^']*')/version('${ver}')/" \
    "${ROOT}/age/impl/prolog/sayableage.pl"
