#!/usr/bin/env bash
# Run one library's dotnet test suite (the solution in the current directory).
set -euo pipefail

dotnet test
