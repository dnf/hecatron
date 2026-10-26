#!/usr/bin/env bash
# Build one library's COBOL tests with CMake and run them with ctest.
set -euo pipefail

cmake -B build
cmake --build build
(cd build && ctest --output-on-failure)
