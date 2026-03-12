#!/usr/bin/env bash
# Update VB.NET version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/Public Const Version As String = "[^"]*"/Public Const Version As String = "'"${ver}"'"/' \
    "${ROOT}/age/impl/vbnet/SayableAge/SayableAge.vb"
_sed_i 's/Public Const Version As String = "[^"]*"/Public Const Version As String = "'"${ver}"'"/' \
    "${ROOT}/age/impl/vbnet/SayableAge/Version.vb"
_sed_i 's/<Version>[^<]*<\/Version>/<Version>'"${ver}"'<\/Version>/' \
    "${ROOT}/age/impl/vbnet/SayableAge/SayableAge.vbproj"
