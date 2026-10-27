#!/usr/bin/env bash
# Check for the Bats a component's .bats tests need (see install.sh).
source "$(dirname "$0")/../_common.sh"

if impl_has bash -name '*.bats' && ! has bats; then
    problem "bats not installed (a component has .bats tests)"
fi
doctor_done
