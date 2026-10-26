#!/usr/bin/env bash
# Run the library's Wren test script.
set -euo pipefail
wren_cli "test_format_$1.wren"
