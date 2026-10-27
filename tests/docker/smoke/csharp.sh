# Smoke test for csharp: build and run a small program in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
dotnet new console --output hello >/dev/null && dotnet run --project hello
