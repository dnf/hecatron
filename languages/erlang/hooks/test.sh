#!/usr/bin/env bash
# Default test hook: rebar3's EUnit tests, and Common Test suites when there are any.
source "$(dirname "$0")/../../_hooks.sh"
[ -f rebar.config ] || unknown "no rebar.config"
run rebar3 eunit
if exists 'test/*_SUITE.erl'; then run rebar3 ct; fi
