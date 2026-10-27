# Smoke test for pascal: build and run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf 'program hello;\nbegin\n  writeln(6 * 7);\nend.\n' > hello.pas
fpc -Sew -ohello hello.pas >/dev/null
[ "$(./hello)" = "42" ]
