#!/usr/bin/env bash
# Check Erlang/OTP and rebar3: on Ubuntu, the OTP build install.sh installs
# (the version config.json pins); elsewhere, say when the distribution's is
# older than current libraries and Gleam's standard library need.
source "$(dirname "$0")/../_common.sh"

if otp_builds_url >/dev/null; then
    _version="$(default_version)"
    otp_of "$(toolchain_home erlang "${_version}")/bin/erl" >/dev/null ||
        problem "Erlang/OTP ${_version} not installed or doesn't start"
fi
if ! has erl; then
    problem "Erlang/OTP not installed (erl not found)"
elif ! otp_at_least 26; then
    note "erl is OTP $(otp_of "$(command -v erl)" | cut -f1); Gleam's standard library and many current libraries need 26 or newer"
fi
has rebar3 || problem "rebar3 not installed"
doctor_done
