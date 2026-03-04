#!/usr/bin/env bash
# Update Groovy package metadata
source "$(dirname "$0")/../_common.sh"

_set_metadata_gradle_groovy "${ROOT}/sayable-age/src/groovy/build.gradle"
