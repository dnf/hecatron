#!/usr/bin/env bash
# Update PHP version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i "s/public const VERSION = '[^']*'/public const VERSION = '${ver}'/" \
    "${ROOT}/sayable-age/src/php/src/SayableAge.php"
