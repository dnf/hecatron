#!/usr/bin/env bash
# Update Dart package metadata
source "$(dirname "$0")/../_common.sh"

_sed_i "s/^description: .*/description: ${PT_DESCRIPTION}/" \
    "${ROOT}/src/dart/pubspec.yaml"
