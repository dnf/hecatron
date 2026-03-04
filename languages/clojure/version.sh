#!/usr/bin/env bash
# Update Clojure version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/(def version "[^"]*")/(def version "'"${ver}"'")/' \
    "${ROOT}/sayable-age/src/clojure/src/sayableage/formatter.clj"
