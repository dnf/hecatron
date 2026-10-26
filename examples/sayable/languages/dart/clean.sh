#!/usr/bin/env bash
# Remove one library's Dart tool cache and legacy package map.
set -euo pipefail

rm -rf .dart_tool .packages
