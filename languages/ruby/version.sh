#!/usr/bin/env bash
# Update Ruby version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i "s/VERSION = '[^']*'/VERSION = '${ver}'/" \
    "${ROOT}/src/ruby/lib/parent_time/version.rb"
