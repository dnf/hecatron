#!/usr/bin/env bash
# Remove one library's D build outputs.
set -euo pipefail

rm -rf .dub ./*.o test_runner ./*-test-library
