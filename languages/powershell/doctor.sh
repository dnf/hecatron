#!/usr/bin/env bash
# Check for the PowerShell modules the default hooks use (see install.sh).
source "$(dirname "$0")/../_common.sh"

if has pwsh; then
    for _module in Pester PSScriptAnalyzer; do
        pwsh -NoProfile -Command "if (-not (Get-Module -ListAvailable -Name ${_module})) { exit 1 }" >/dev/null 2>&1 ||
            problem "PowerShell module ${_module} is not installed"
    done
fi
doctor_done
