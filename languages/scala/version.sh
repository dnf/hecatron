#!/usr/bin/env bash
# Update Scala version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/val Version = "[^"]*"/val Version = "'"${ver}"'"/' \
    "${ROOT}/src/scala/src/main/scala/parenttime/ParentTime.scala"
_sed_i 's/^version = "[^"]*"/version = "'"${ver}"'"/' \
    "${ROOT}/src/scala/build.gradle.kts"
