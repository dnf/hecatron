#!/usr/bin/env bash
# Default format hook: dune's ocamlformat rules, where configured (see format_ocaml in languages/_format.sh).
# shellcheck source=_dune.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_dune.sh"
source "$(dirname "$0")/../../_format.sh"
format_ocaml
