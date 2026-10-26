#!/usr/bin/env bash
# Run one library's Zig tests (called from <library>/impl/zig).
set -euo pipefail

zig build test --summary all
