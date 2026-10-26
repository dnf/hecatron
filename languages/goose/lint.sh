#!/usr/bin/env bash
# Typecheck one Goose library with zero diagnostics ($1: age, date, duration or time).
set -euo pipefail
bash lint.sh
