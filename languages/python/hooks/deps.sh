#!/usr/bin/env bash
# Default deps hook: uv sync, --locked with a uv.lock (see deps_python in languages/_deps.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_deps.sh"
deps_python
