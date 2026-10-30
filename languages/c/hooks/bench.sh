#!/usr/bin/env bash
# Default bench hook: a CMake bench target or Google Benchmark programs, Meson's
# benchmarks, a Makefile's bench target.
source "$(dirname "$0")/../../_hooks.sh"
native_hook bench
