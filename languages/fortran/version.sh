#!/usr/bin/env bash
# Update Fortran version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/PT_VERSION = "[^"]*"/PT_VERSION = "'"${ver}"'"/' \
    "${ROOT}/sayable-age/src/fortran/src/sayableage.f90"
