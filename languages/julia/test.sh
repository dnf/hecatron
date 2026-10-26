#!/usr/bin/env bash
# Run one library's Julia package tests.
set -euo pipefail

julia --project -e 'using Pkg; Pkg.test()'
