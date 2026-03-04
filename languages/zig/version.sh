#!/usr/bin/env bash
# Update Zig version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/pub const version = "[^"]*"/pub const version = "'"${ver}"'"/' \
    "${ROOT}/sayable-age/src/zig/src/sayableage.zig"
_sed_i 's/\.version = "[^"]*"/.version = "'"${ver}"'"/' \
    "${ROOT}/sayable-age/src/zig/build.zig.zon"
