#!/usr/bin/env bash
# Update Gleam package metadata
source "$(dirname "$0")/../_common.sh"

_sed_i 's/^description = "[^"]*"/description = "'"${PT_DESCRIPTION}"'"/' \
    "${ROOT}/age/src/gleam/gleam.toml"
_sed_i 's/^licences = \["[^"]*"\]/licences = ["'"${PT_LICENSE}"'"]/' \
    "${ROOT}/age/src/gleam/gleam.toml"
