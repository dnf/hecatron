#!/usr/bin/env bash
# Update C version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/#define PARENTTIME_VERSION "[^"]*"/#define PARENTTIME_VERSION "'"${ver}"'"/' \
    "${ROOT}/codegen/targets/c/emitter.py"
