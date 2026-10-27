# Shared by the Haskell hooks (sourced, not run as a hook): Stack when the
# directory has a stack.yaml, else cabal for a package (*.cabal) or project
# (cabal.project).
# shellcheck shell=bash
source "$(dirname "$0")/../../_hooks.sh"

is_stack() { [ -f stack.yaml ]; }
is_cabal() { exists '*.cabal' || [ -f cabal.project ]; }
haskell_project() { is_stack || is_cabal || unknown "no stack.yaml, *.cabal or cabal.project"; }
