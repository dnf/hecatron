#!/usr/bin/env bash
# Update Julia version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/^version = "[^"]*"/version = "'"${ver}"'"/' \
    "${ROOT}/age/impl/julia/Project.toml"
_sed_i 's/const VERSION = "[^"]*"/const VERSION = "'"${ver}"'"/' \
    "${ROOT}/age/impl/julia/src/SayableAge.jl"
