#!/usr/bin/env bash
# Update Go version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/const Version = "[^"]*"/const Version = "'"${ver}"'"/' \
    "${ROOT}/age/src/go/sayableage.go"
