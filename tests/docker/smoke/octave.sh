# Smoke test for octave: run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf 'disp(6 * 7)\n' > hello.m
[ "$(octave --no-gui --no-window-system --quiet hello.m)" = "42" ]
