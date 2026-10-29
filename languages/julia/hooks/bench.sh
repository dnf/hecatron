#!/usr/bin/env bash
# Default bench hook: BenchmarkTools' SUITE in benchmark/benchmarks.jl
# (see bench_julia in languages/_bench.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_bench.sh"
bench_julia
