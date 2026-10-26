#!/usr/bin/env bash
# Remove one library's TypeScript build output (not node_modules or package-lock.json).
set -euo pipefail

rm -rf dist
