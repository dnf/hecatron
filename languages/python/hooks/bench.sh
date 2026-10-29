#!/usr/bin/env bash
# Default bench hook: pytest --benchmark-only, where pytest-benchmark is a dependency
# (see bench_python in languages/_bench.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_bench.sh"
bench_python
