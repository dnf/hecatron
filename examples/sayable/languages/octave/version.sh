#!/usr/bin/env bash
# Update Octave version in source
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i "s/v = '[^']*'/v = '${ver}'/" \
    "${ROOT}/age/impl/octave/version.m"
