#!/usr/bin/env bash
# Update C# version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/public const string Version = "[^"]*"/public const string Version = "'"${ver}"'"/' \
    "${ROOT}/age/impl/csharp/SayableAge/SayableAge.cs"
_sed_i 's/<Version>[^<]*<\/Version>/<Version>'"${ver}"'<\/Version>/' \
    "${ROOT}/age/impl/csharp/SayableAge/SayableAge.csproj"
