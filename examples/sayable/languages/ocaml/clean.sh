#!/usr/bin/env bash
# Remove one library's dune build outputs (not the local opam switch).
set -euo pipefail

rm -rf _build
