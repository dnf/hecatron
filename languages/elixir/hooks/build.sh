#!/usr/bin/env bash
# Default build hook: mix compile.
source "$(dirname "$0")/../../_hooks.sh"
[ -f mix.exs ] || unknown "no mix.exs"
mix_deps
run mix compile
