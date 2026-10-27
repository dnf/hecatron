# Smoke test for raku: run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf 'say 6 * 7;\n' > hello.raku
raku -c hello.raku
[ "$(raku hello.raku)" = "42" ]
