#!/usr/bin/env bash
# Update Java package metadata
source "$(dirname "$0")/../_common.sh"

_set_metadata_gradle_kts "${ROOT}/age/src/java/build.gradle.kts"
