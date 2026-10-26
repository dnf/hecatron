#!/usr/bin/env bash
# Update PowerShell version in source
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i "s/\\\$script:VERSION = '[^']*'/\$script:VERSION = '${ver}'/" \
    "${ROOT}/age/impl/powershell/SayableAge.psm1"
