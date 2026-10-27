#!/usr/bin/env bash
# Default clean hook: what building and testing leave (dist/, build/,
# *.egg-info, caches), never the virtualenv.
source "$(dirname "$0")/../../_hooks.sh"
run rm -rf dist build .pytest_cache .ruff_cache ./*.egg-info src/*.egg-info
find . -name __pycache__ -type d -not -path './.venv/*' -prune -exec rm -rf {} +
