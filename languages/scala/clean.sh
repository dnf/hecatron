#!/usr/bin/env bash
# Remove one library's Gradle build outputs and cache.
set -euo pipefail

rm -rf build .gradle
