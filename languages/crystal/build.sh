#!/usr/bin/env bash
# Type-check one library's Crystal sources (a shard with no executable targets, so there is nothing to link).
set -euo pipefail

shards install
crystal build --no-codegen "src/sayable_${1}.cr"
