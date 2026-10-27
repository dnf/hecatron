#!/usr/bin/env bash
# Default build hook: the sdist and wheel, into dist/.
source "$(dirname "$0")/../../_hooks.sh"
if [ -f pyproject.toml ] || [ -f setup.py ]; then
    run uv build
else
    skip "no pyproject.toml or setup.py to build"
fi
