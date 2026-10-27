# Smoke test for tcl: run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf 'puts [expr {6 * 7}]\n' > hello.tcl
[ "$(tclsh hello.tcl)" = "42" ]
