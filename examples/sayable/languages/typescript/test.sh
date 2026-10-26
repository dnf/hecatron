#!/usr/bin/env bash
# Install dependencies, build where the library has a build step, and run the npm tests.
set -euo pipefail
npm install
if [ "$1" = "age" ]; then npm run build; fi
npm test
