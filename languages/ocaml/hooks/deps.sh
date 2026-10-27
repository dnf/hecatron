#!/usr/bin/env bash
# Default deps hook: opam install --deps-only (see deps_ocaml in languages/_deps.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_deps.sh"
deps_ocaml
