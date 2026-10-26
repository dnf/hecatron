#!/usr/bin/env bash
# Run the library's Go tests.
set -euo pipefail
go test -v ./...
