#!/usr/bin/env bash
# Default lint hook: Dialyzer, through rebar3 (its first run builds the
# OTP lookup table, which rebar3 caches).
source "$(dirname "$0")/../../_hooks.sh"
[ -f rebar.config ] || unknown "no rebar.config"
run rebar3 dialyzer
