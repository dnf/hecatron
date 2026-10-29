#!/usr/bin/env bash
# Default bench hook: each BenchmarkDotNet project, built for release
# (see bench_dotnet in languages/_bench.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_bench.sh"
bench_dotnet
