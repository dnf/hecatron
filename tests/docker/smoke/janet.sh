# Smoke test for janet: run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf '(print (sum (range 1 10)))\n' > hello.janet
_out="$(janet hello.janet)"
[ "${_out}" = "45" ] || { echo "expected 45, got: ${_out}" >&2; exit 1; }
echo "${_out}"
