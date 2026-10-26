#!/usr/bin/env bash
# Configure and build one library with CMake.
set -euo pipefail

cmake -B build
cmake --build build
