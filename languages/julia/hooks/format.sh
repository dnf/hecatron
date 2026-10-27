#!/usr/bin/env bash
# Default format hook: JuliaFormatter, where configured (see format_julia in languages/_format.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_format.sh"
format_julia
