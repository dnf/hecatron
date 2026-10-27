#!/usr/bin/env bash
# Default clean hook: rebar3 clean in the default and test profiles (the
# project's own build output; dependencies stay).
source "$(dirname "$0")/../../_hooks.sh"
[ -f rebar.config ] || unknown "no rebar.config"
run rebar3 clean
run rebar3 as test clean
