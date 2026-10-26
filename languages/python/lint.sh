#!/usr/bin/env bash
# Lint and typecheck one Python library ($1: age, date, duration or time).
set -euo pipefail
uv run --extra dev ruff check .
uv run --extra dev pyright
