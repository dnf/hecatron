#!/usr/bin/env bash
# Update Scala version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/val Version = "[^"]*"/val Version = "'"${ver}"'"/' \
    "${ROOT}/age/impl/scala/src/main/scala/sayableage/SayableAge.scala"
_sed_i 's/^version = "[^"]*"/version = "'"${ver}"'"/' \
    "${ROOT}/age/impl/scala/build.gradle.kts"
