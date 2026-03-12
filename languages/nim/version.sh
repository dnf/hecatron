#!/usr/bin/env bash
# Update Nim version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/const Version\* = "[^"]*"/const Version* = "'"${ver}"'"/' \
    "${ROOT}/age/impl/nim/src/sayable_age/formatter.nim"
_sed_i 's/^version  *= "[^"]*"/version       = "'"${ver}"'"/' \
    "${ROOT}/age/impl/nim/sayable_age.nimble"
