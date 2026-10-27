#!/usr/bin/env bash
# Default test hook: pytest, in the project's environment (with every extra,
# where test dependencies usually are) through uv, else on its own.
source "$(dirname "$0")/../../_hooks.sh"
if [ -f pyproject.toml ] || [ -f setup.py ] || [ -f setup.cfg ]; then
    run uv run --all-extras --with pytest pytest
else
    run uv run --with pytest pytest
fi
