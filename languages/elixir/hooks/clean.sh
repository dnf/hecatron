#!/usr/bin/env bash
# Default clean hook: mix clean in the dev and test environments (the
# project's own build output; fetched and compiled dependencies stay).
source "$(dirname "$0")/../../_hooks.sh"
[ -f mix.exs ] || unknown "no mix.exs"
run mix clean
run env MIX_ENV=test mix clean
