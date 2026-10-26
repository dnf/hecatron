#!/usr/bin/env bash
# Compile one library with warnings as errors.
set -euo pipefail

mix compile --warnings-as-errors
