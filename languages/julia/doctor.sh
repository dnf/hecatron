#!/usr/bin/env bash
# Check for JuliaFormatter where a component configures it (see install.sh).
source "$(dirname "$0")/../_common.sh"

if has julia && julia_formatter_missing; then
    problem "JuliaFormatter ${_JULIA_FORMATTER_VERSION} not installed (a component has a .JuliaFormatter.toml)"
fi
doctor_done
