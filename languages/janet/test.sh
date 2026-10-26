#!/usr/bin/env bash
# Run one library's Janet test script.
set -euo pipefail

janet "test_format_${1}.janet"
