#!/usr/bin/env bash
# Check for Hex, which mix needs to fetch dependencies (see install.sh).
source "$(dirname "$0")/../_common.sh"

if has mix && impl_has elixir -name mix.exs && ! mix hex.info >/dev/null 2>&1; then
    problem "Hex is not installed (mix local.hex)"
fi
doctor_done
