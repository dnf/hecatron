#!/usr/bin/env bash
# Update Elixir version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/@version "[^"]*"/@version "'"${ver}"'"/' \
    "${ROOT}/sayable-age/src/elixir/lib/sayable_age/formatter.ex"
_sed_i 's/version: "[^"]*"/version: "'"${ver}"'"/' \
    "${ROOT}/sayable-age/src/elixir/mix.exs"
