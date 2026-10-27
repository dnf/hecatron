# Smoke test for vbnet: build and run a small program in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
dotnet new console --language "VB" --output hello >/dev/null && dotnet run --project hello
