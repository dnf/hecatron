#!/usr/bin/env bash
# Update Scala package metadata
source "$(dirname "$0")/../_common.sh"

_set_metadata_gradle_kts "${ROOT}/age/src/scala/build.gradle.kts"
