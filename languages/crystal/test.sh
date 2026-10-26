#!/usr/bin/env bash
# Install shards and run one library's native Crystal specs.
set -euo pipefail

shards install
crystal spec
