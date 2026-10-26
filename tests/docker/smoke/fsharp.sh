# Smoke test for fsharp: build and run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
dotnet new console --language "F#" --output hello >/dev/null && dotnet run --project hello
