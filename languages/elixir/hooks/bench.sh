#!/usr/bin/env bash
# Default bench hook: bench/*.exs through mix run, where Benchee is a dependency
# (see bench_elixir in languages/_bench.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_bench.sh"
bench_elixir
