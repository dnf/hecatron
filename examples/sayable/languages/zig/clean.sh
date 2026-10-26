#!/usr/bin/env bash
# Remove one library's zig build outputs and cache.
set -euo pipefail

rm -rf zig-out .zig-cache
