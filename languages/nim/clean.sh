#!/usr/bin/env bash
# Remove one library's Nim compiler caches.
set -euo pipefail

find . -type d -name nimcache -exec rm -rf {} +
