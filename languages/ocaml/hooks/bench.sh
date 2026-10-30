#!/usr/bin/env bash
# Default bench hook: dune build @bench, where a dune file defines the alias (see bench_ocaml in languages/_bench.sh).
# shellcheck source=_dune.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_dune.sh"
source "$(dirname "$0")/../../_bench.sh"
bench_ocaml
