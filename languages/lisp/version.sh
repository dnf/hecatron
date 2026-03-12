#!/usr/bin/env bash
# Update Common Lisp version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/^(defvar +version+ "[^"]*"/(defvar +version+ "'"${ver}"'"/' \
    "${ROOT}/age/impl/lisp/formatter.lisp"
_sed_i 's/:version "[^"]*"/:version "'"${ver}"'"/' \
    "${ROOT}/age/impl/lisp/sayable-age.asd"
