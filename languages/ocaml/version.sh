#!/usr/bin/env bash
# Update OCaml version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/^let version = "[^"]*"/let version = "'"${ver}"'"/' \
    "${ROOT}/src/ocaml/lib/formatter.ml"
_sed_i 's/^version: "[^"]*"/version: "'"${ver}"'"/' \
    "${ROOT}/src/ocaml/parent_time.opam"
_sed_i 's/(version [^)]*)/(version '"${ver}"')/' \
    "${ROOT}/src/ocaml/dune-project"
