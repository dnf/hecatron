#!/usr/bin/env bash
# Default lint hook: dune's formatting check, which covers the dune files, and
# the OCaml sources too when the project configures ocamlformat (.ocamlformat).
# shellcheck source=_dune.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_dune.sh"
run dune build @fmt
