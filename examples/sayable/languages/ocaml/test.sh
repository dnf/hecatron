#!/usr/bin/env bash
# Build and run the OCaml dune tests for one library.
set -euo pipefail

eval "$(opam env 2>/dev/null)"
dune build
dune runtest
