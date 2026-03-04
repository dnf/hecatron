#!/usr/bin/env bash
# Update Gleam version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/pub const version = "[^"]*"/pub const version = "'"${ver}"'"/' \
    "${ROOT}/src/gleam/src/parenttime/formatter.gleam"
_sed_i 's/^version = "[^"]*"/version = "'"${ver}"'"/' \
    "${ROOT}/src/gleam/gleam.toml"
