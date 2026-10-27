#!/usr/bin/env bash
# Default build hook: rebar3 compile.
source "$(dirname "$0")/../../_hooks.sh"
[ -f rebar.config ] || unknown "no rebar.config"
run rebar3 compile
