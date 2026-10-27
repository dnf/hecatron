#!/usr/bin/env bash
# Check for the zef a component's META6.json needs (see install.sh).
source "$(dirname "$0")/../_common.sh"

if impl_has raku -name META6.json && ! has zef; then
    problem "zef not installed (a component has a META6.json)"
fi
doctor_done
