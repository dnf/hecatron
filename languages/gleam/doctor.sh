#!/usr/bin/env bash
# Check for the Erlang/OTP that Gleam's compiled code runs on.
source "$(dirname "$0")/../_common.sh"

has erl || problem "Erlang/OTP not installed (gleam build, run and test need erl)"
doctor_done
