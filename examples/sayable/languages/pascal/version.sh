#!/usr/bin/env bash
# Update Pascal version in source
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i "s/SayableAgeVersion = '[^']*'/SayableAgeVersion = '${ver}'/" \
    "${ROOT}/age/impl/pascal/SayableAge.pas"
