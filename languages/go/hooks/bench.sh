#!/usr/bin/env bash
# Default bench hook: the module's Benchmark functions (go test -run '^$' -bench .)
# (see bench_go in languages/_bench.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_bench.sh"
bench_go
