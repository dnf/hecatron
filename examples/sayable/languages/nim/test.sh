#!/usr/bin/env bash
# Run one library's Nim tests (called from <library>/impl/nim).
set -euo pipefail

nimble test -y
