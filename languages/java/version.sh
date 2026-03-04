#!/usr/bin/env bash
# Update Java version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/public static final String VERSION = "[^"]*"/public static final String VERSION = "'"${ver}"'"/' \
    "${ROOT}/src/java/src/main/java/parenttime/ParentTime.java"
_sed_i 's/^version = "[^"]*"/version = "'"${ver}"'"/' \
    "${ROOT}/src/java/build.gradle.kts"
