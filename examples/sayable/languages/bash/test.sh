#!/usr/bin/env bash
# Run one library's native Bash test suite.
set -euo pipefail

bash "test_format_${1}.sh"
