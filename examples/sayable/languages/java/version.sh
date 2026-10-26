#!/usr/bin/env bash
# Update Java version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/public static final String VERSION = "[^"]*"/public static final String VERSION = "'"${ver}"'"/' \
    "${ROOT}/age/impl/java/src/main/java/sayableage/SayableAge.java"
_sed_i 's/^version = "[^"]*"/version = "'"${ver}"'"/' \
    "${ROOT}/age/impl/java/build.gradle.kts"
