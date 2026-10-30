#!/usr/bin/env bash
# Default bench hook: the Rakefile's bench or benchmark task
# (see bench_ruby in languages/_bench.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_bench.sh"
bench_ruby
