#!/usr/bin/env bash
# Remove one library's Crystal build outputs.
set -euo pipefail

rm -rf bin lib .shards
