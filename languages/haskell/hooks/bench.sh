#!/usr/bin/env bash
# Default bench hook: every benchmark suite, through Stack or cabal (see bench_haskell in languages/_bench.sh).
# shellcheck source=_haskell.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_haskell.sh"
source "$(dirname "$0")/../../_bench.sh"
bench_haskell
