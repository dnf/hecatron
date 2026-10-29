#!/usr/bin/env bash
# Default bench hook: the build's bench step, where build.zig defines one
# (see bench_zig in languages/_bench.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_bench.sh"
bench_zig
