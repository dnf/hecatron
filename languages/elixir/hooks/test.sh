#!/usr/bin/env bash
# Default test hook: mix test.
source "$(dirname "$0")/../../_hooks.sh"
[ -f mix.exs ] || unknown "no mix.exs"
mix_deps
run mix test
