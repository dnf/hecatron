#!/usr/bin/env bash
# Check for the build tools and linter the components need (see install.sh).
source "$(dirname "$0")/../_common.sh"

while IFS= read -r _tool; do
    [ -n "${_tool}" ] && problem "${_tool} not installed (a component needs it)"
done < <(perl_tools_missing)
doctor_done
