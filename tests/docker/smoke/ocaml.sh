# Smoke test for ocaml: build and run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
has_dune() { command -v dune >/dev/null 2>&1; }
has_dune || eval "$(opam env)"
# Without an .ocamlformat the installer installs no ocamlformat, so @fmt
# checks the dune files only.
printf '(lang dune 3.0)\n\n(formatting\n (enabled_for dune))\n' > dune-project
printf '(executable\n (name hello))\n' > dune
printf 'let () = print_endline (string_of_int (6 * 7))\n' > hello.ml
dune build --display=quiet ./hello.exe
[ "$(./_build/default/hello.exe)" = "42" ]
dune build --display=quiet @fmt
