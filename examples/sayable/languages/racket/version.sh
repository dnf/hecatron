#!/usr/bin/env bash
# Update Racket version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/(define version "[^"]*")/(define version "'"${ver}"'")/' \
    "${ROOT}/age/impl/racket/format-age.rkt" \
    "${ROOT}/age/impl/racket/info.rkt"
