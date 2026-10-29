#!/usr/bin/env bash
# Default bench hook: PHPBench, where composer.json requires it
# (see bench_php in languages/_bench.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_bench.sh"
bench_php
