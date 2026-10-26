#!/usr/bin/env bash
# Lint one Go library ($1: age, date, duration or time).
set -euo pipefail
golangci-lint run
