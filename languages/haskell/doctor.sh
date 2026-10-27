#!/usr/bin/env bash
# Check for the Stack or HLint the components use (see install.sh).
source "$(dirname "$0")/../_common.sh"
# shellcheck source=_tools.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_tools.sh"

while IFS= read -r _tool; do
    problem "${_tool} not installed (a component uses it)"
done < <(haskell_tools_missing)
doctor_done
