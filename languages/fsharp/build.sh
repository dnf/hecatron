#!/usr/bin/env bash
# Build one library's solution with dotnet (the solution in the current directory).
set -euo pipefail

dotnet build
