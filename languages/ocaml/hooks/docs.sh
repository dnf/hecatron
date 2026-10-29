#!/usr/bin/env bash
# Default docs hook: odoc through dune build @doc (see docs_ocaml in languages/_docs.sh).
# shellcheck source=_dune.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_dune.sh"
source "$(dirname "$0")/../../_docs.sh"
docs_ocaml
