#!/usr/bin/env bash
# Install dependencies, then bundle with tsup (age) or type-check (libraries with no bundle config).
set -euo pipefail

npm install
if [ "$1" = "age" ]; then npm run build; else npx tsc --noEmit; fi
