#!/usr/bin/env bash
# Lint, run one library's Goose tests, then build and run them natively.
set -euo pipefail

bash lint.sh
goose "tests/test_format_${1}.goose"
bash build.sh
"./build/test_format_${1}"
