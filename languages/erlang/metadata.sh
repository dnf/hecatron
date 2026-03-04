#!/usr/bin/env bash
# Update Erlang package metadata
source "$(dirname "$0")/../_common.sh"

_sed_i 's/{description, "[^"]*"}/{description, "'"${PT_DESCRIPTION}"'"}/' \
    "${ROOT}/src/erlang/src/parenttime.app.src"
_sed_i 's/{licenses, \["[^"]*"\]}/{licenses, ["'"${PT_LICENSE}"'"]}/' \
    "${ROOT}/src/erlang/src/parenttime.app.src"
