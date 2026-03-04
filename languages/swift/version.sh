#!/usr/bin/env bash
# Update Swift version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/public static let version = "[^"]*"/public static let version = "'"${ver}"'"/' \
    "${ROOT}/age/src/swift/Sources/SayableAge/SayableAge.swift"
