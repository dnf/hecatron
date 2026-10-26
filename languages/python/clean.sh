#!/usr/bin/env bash
# Remove one library's Python bytecode and tool caches, keeping the virtualenv.
set -euo pipefail

find . -type d -name __pycache__ -not -path './.venv/*' -exec rm -rf {} + 2>/dev/null || true
rm -rf .pytest_cache .ruff_cache
