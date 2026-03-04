#!/usr/bin/env bash
# Update Groovy version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/static final String VERSION = "[^"]*"/static final String VERSION = "'"${ver}"'"/' \
    "${ROOT}/src/groovy/src/main/groovy/parenttime/ParentTime.groovy"
_sed_i "s/^version = '[^']*'/version = '${ver}'/" \
    "${ROOT}/src/groovy/build.gradle"
