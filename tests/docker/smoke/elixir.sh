# Smoke test for elixir: create a mix project in a scratch directory, run a
# function from it, and check its formatting as the default lint hook does.
# shellcheck shell=bash
set -euo pipefail
mix new hello >/dev/null && cd hello
cat > lib/hello.ex <<'EOF'
defmodule Hello do
  def answer, do: Enum.sum(1..9)
end
EOF
mix compile --warnings-as-errors >/dev/null
_out="$(mix run -e 'IO.puts(Hello.answer())')"
[ "${_out}" = "45" ] || { echo "expected 45, got: ${_out}" >&2; exit 1; }
echo "${_out}"
mix format --check-formatted
