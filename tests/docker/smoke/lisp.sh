# Smoke test for lisp: run a hello world with SBCL in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf '(format t "~D~%%" (loop for i from 1 to 9 sum i))\n' > hello.lisp
_out="$(sbcl --script hello.lisp)"
[ "${_out}" = "45" ] || { echo "expected 45, got: ${_out}" >&2; exit 1; }
echo "${_out}"
