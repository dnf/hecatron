#!/usr/bin/env bash
# Run the library's pytest suite against its source tree.
set -euo pipefail
PYTHONPATH=src pytest tests/ -v
