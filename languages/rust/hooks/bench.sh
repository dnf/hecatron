#!/usr/bin/env bash
# Default bench hook: cargo bench, where there are benchmarks
# (see bench_rust in languages/_bench.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_bench.sh"
bench_rust
