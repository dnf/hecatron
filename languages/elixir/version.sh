#!/usr/bin/env bash
# Update Elixir version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/@version "[^"]*"/@version "'"${ver}"'"/' \
    "${ROOT}/src/elixir/lib/parent_time/formatter.ex"
_sed_i 's/version: "[^"]*"/version: "'"${ver}"'"/' \
    "${ROOT}/src/elixir/mix.exs"
