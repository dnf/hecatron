#!/usr/bin/env bash
# Update Elm version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/^    "[0-9][^"]*"/    "'"${ver}"'"/' \
    "${ROOT}/age/src/elm/src/SayableAge/Formatter.elm"
