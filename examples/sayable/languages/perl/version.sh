#!/usr/bin/env bash
# Update Perl version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i "s/our \$VERSION = '[^']*'/our \$VERSION = '${ver}'/" \
    "${ROOT}/age/impl/perl/lib/SayableAge/Formatter.pm"
