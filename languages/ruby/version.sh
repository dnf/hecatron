#!/usr/bin/env bash
# Update Ruby version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i "s/VERSION = '[^']*'/VERSION = '${ver}'/" \
    "${ROOT}/sayable-age/src/ruby/lib/sayable_age/version.rb"
