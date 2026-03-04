#!/usr/bin/env bash
# Update Dart package metadata
source "$(dirname "$0")/../_common.sh"

_sed_i "s/^description: .*/description: ${PT_DESCRIPTION}/" \
    "${ROOT}/age/src/dart/pubspec.yaml"
