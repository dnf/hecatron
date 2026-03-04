#!/usr/bin/env bash
# Update C++ version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/constexpr const char\* VERSION = "[^"]*"/constexpr const char* VERSION = "'"${ver}"'"/' \
    "${ROOT}/src/cpp/include/parenttime/types.hpp"
_sed_i 's/VERSION [0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*/VERSION '"${ver}"'/' \
    "${ROOT}/src/cpp/CMakeLists.txt"
