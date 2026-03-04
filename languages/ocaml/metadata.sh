#!/usr/bin/env bash
# Update OCaml package metadata
source "$(dirname "$0")/../_common.sh"

_sed_i 's/^synopsis: "[^"]*"/synopsis: "'"${PT_DESCRIPTION}"'"/' \
    "${ROOT}/sayable-age/src/ocaml/sayable_age.opam"
_sed_i 's/^maintainer: \["[^"]*"\]/maintainer: ["'"${PT_AUTHOR}"'"]/' \
    "${ROOT}/sayable-age/src/ocaml/sayable_age.opam"
