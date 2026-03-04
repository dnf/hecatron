#!/usr/bin/env bash
# Update Kotlin version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/public const val VERSION: String = "[^"]*"/public const val VERSION: String = "'"${ver}"'"/' \
    "${ROOT}/sayable-age/src/kotlin/src/main/kotlin/sayableage/SayableAge.kt"
_sed_i 's/^version = "[^"]*"/version = "'"${ver}"'"/' \
    "${ROOT}/sayable-age/src/kotlin/build.gradle.kts"
