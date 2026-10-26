#!/usr/bin/env bash
# Configure, build and run one library's CTest suite.
set -euo pipefail

cmake -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build
(cd build && ctest --output-on-failure)
