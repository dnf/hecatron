#!/usr/bin/env bash
# Default lint hook: the formatter's check where the project configures it
# (.formatter.exs), and Credo where it's a dependency.
source "$(dirname "$0")/../../_hooks.sh"
[ -f mix.exs ] || unknown "no mix.exs"
_formatter=0
_credo=0
[ -f .formatter.exs ] && _formatter=1
grep -q ':credo' mix.exs && _credo=1
[ "${_formatter}" -eq 1 ] || [ "${_credo}" -eq 1 ] || skip "no .formatter.exs and no Credo dependency"
mix_deps
if [ "${_formatter}" -eq 1 ]; then run mix format --check-formatted; fi
if [ "${_credo}" -eq 1 ]; then run mix credo --strict; fi
