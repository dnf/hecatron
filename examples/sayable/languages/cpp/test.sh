#!/usr/bin/env bash
# Build with CMake and run the library's ctest suite.
set -euo pipefail
cmake -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build
(cd build && ctest --output-on-failure)
