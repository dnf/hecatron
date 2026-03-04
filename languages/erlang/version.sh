#!/usr/bin/env bash
# Update Erlang version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/-define(VERSION, "[^"]*")/-define(VERSION, "'"${ver}"'")/' \
    "${ROOT}/src/erlang/include/parenttime.hrl"
_sed_i 's/{vsn, "[^"]*"}/{vsn, "'"${ver}"'"}/' \
    "${ROOT}/src/erlang/src/parenttime.app.src"
