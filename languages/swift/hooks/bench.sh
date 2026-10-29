#!/usr/bin/env bash
# Default bench hook: swift package benchmark, where the package depends on package-benchmark
# (see bench_swift in languages/_bench.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_bench.sh"
bench_swift
