#!/usr/bin/env bash
# Update Pharo version in source
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i "s/Version := '[^']*'/Version := '${ver}'/" \
    "${ROOT}/age/impl/pharo/SayableAge.st"
