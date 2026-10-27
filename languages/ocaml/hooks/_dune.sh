# Shared by the OCaml hooks (sourced, not run as a hook): dune, from the
# current opam switch.
# shellcheck shell=bash
source "$(dirname "$0")/../../_hooks.sh"

[ -f dune-project ] || unknown "no dune-project"
has dune || eval "$(opam env 2>/dev/null)"
