#!/usr/bin/env bash
# Update Janet version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/^(def version "[^"]*")/(def version "'"${ver}"'")/' \
    "${ROOT}/age/impl/janet/sayableage.janet"
_sed_i 's/:version "[^"]*"/:version "'"${ver}"'"/' \
    "${ROOT}/age/impl/janet/project.janet"
