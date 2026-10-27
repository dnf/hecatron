# Smoke test for prolog: run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf ':- initialization(main, main).\nmain :- X is 6 * 7, format("~w~n", [X]).\n' > hello.pl
[ "$(swipl hello.pl)" = "42" ]
