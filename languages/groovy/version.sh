#!/usr/bin/env bash
# Update Groovy version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/static final String VERSION = "[^"]*"/static final String VERSION = "'"${ver}"'"/' \
    "${ROOT}/sayable-age/src/groovy/src/main/groovy/sayableage/SayableAge.groovy"
_sed_i "s/^version = '[^']*'/version = '${ver}'/" \
    "${ROOT}/sayable-age/src/groovy/build.gradle"
