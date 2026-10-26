#!/usr/bin/env bash
# Update Kotlin package metadata
source "$(dirname "$0")/../_common.sh"

_set_metadata_gradle_kts "${ROOT}/age/impl/kotlin/build.gradle.kts"
