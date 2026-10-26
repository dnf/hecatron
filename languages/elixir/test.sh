#!/usr/bin/env bash
# Compile with warnings as errors and run one library's ExUnit suite.
set -euo pipefail

mix compile --warnings-as-errors
mix test
