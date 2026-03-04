#!/usr/bin/env bash
# Update F# version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/\[<Literal>\] Version = "[^"]*"/[<Literal>] Version = "'"${ver}"'"/' \
    "${ROOT}/age/src/fsharp/SayableAge/Formatter.fs"
_sed_i 's/<Version>[^<]*<\/Version>/<Version>'"${ver}"'<\/Version>/' \
    "${ROOT}/age/src/fsharp/SayableAge/SayableAge.fsproj"
