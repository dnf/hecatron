#!/usr/bin/env bash
# Update Crystal version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/VERSION = "[^"]*"/VERSION = "'"${ver}"'"/' \
    "${ROOT}/age/impl/crystal/src/sayable_age/version.cr"
_sed_i 's/^version: .*/version: '"${ver}"'/' \
    "${ROOT}/age/impl/crystal/shard.yml"
