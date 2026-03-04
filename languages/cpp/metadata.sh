#!/usr/bin/env bash
# Update C++ package metadata
source "$(dirname "$0")/../_common.sh"

_sed_i 's|DESCRIPTION "[^"]*"|DESCRIPTION "'"${PT_DESCRIPTION}"'"|' \
    "${ROOT}/src/cpp/CMakeLists.txt"
_sed_i 's|HOMEPAGE_URL "[^"]*"|HOMEPAGE_URL "'"${PT_REPOSITORY}"'"|' \
    "${ROOT}/src/cpp/CMakeLists.txt"
