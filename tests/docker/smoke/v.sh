# Smoke test for v: build and run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf "Module {\n\tname: 'hello'\n\tversion: '0.0.1'\n}\n" > v.mod
printf 'module main\n\nfn main() {\n\tprintln(6 * 7)\n}\n' > hello.v
[ "$(v run .)" = "42" ]
v fmt -verify .
v vet .
