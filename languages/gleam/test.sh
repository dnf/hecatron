#!/usr/bin/env bash
# Build and run one library's gleam test suite.
set -euo pipefail

gleam build
gleam test
