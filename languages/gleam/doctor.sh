#!/usr/bin/env bash
# Check for the Erlang/OTP that Gleam's compiled code runs on.
source "$(dirname "$0")/../_common.sh"

if ! has erl; then
    problem "Erlang/OTP not installed (gleam build, run and test need erl)"
elif ! otp_at_least 26; then
    problem "erl is OTP $(otp_of "$(command -v erl)" | cut -f1); Gleam's standard library needs 26 or newer"
fi
doctor_done
