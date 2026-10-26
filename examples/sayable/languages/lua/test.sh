#!/usr/bin/env bash
# Run one library's Lua busted spec.
set -euo pipefail

busted "test_sayable${1}.lua"
