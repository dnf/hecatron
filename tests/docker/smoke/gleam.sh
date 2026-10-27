# Smoke test for gleam: create a project in a scratch directory, run it on the
# BEAM, and check its formatting as the default lint hook does.
# shellcheck shell=bash
set -euo pipefail
gleam new hello >/dev/null && cd hello
cat > src/hello.gleam <<'EOF'
import gleam/int
import gleam/io

pub fn main() {
  [1, 2, 3, 4, 5, 6, 7, 8, 9]
  |> int.sum
  |> int.to_string
  |> io.println
}
EOF
_out="$(gleam run)"
[ "${_out}" = "45" ] || { echo "expected 45, got: ${_out}" >&2; exit 1; }
echo "${_out}"
gleam format --check .
