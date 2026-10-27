#!/usr/bin/env bash
# Check for Pester, which install.sh installs from the PowerShell Gallery.
source "$(dirname "$0")/../_common.sh"

if has pwsh && ! pwsh -NoProfile -Command "if (-not (Get-Module -ListAvailable -Name Pester)) { exit 1 }"; then
    problem "Pester (PowerShell's test framework) is not installed"
fi
doctor_done
