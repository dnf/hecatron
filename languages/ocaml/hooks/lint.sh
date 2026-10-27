#!/usr/bin/env bash
# Default lint hook: with ocamlformat configured (.ocamlformat), dune's
# formatting check over the dune files and OCaml sources; without it, dune
# would still try to format the sources and fail for want of ocamlformat, so
# type-check everything instead (@check, warnings included).
# shellcheck source=_dune.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_dune.sh"
if [ -f .ocamlformat ]; then
    checks_format
    run dune build @fmt
else
    run dune build @check
fi
