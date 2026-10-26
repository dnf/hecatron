#!/usr/bin/env bash
# Build one library with dune.
set -euo pipefail

eval "$(opam env 2>/dev/null)"
dune build
