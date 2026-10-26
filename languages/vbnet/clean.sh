#!/usr/bin/env bash
# Remove one library's dotnet build outputs.
set -euo pipefail

rm -rf ./*/bin ./*/obj
