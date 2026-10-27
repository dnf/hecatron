#!/usr/bin/env bash
# Default test hook: Pester, on every *.Tests.ps1 file here.
# shellcheck source=languages/_hooks.sh
source "$(dirname "$0")/../../_hooks.sh"
find_files -name '*.Tests.ps1'
[ "${#found_files[@]}" -gt 0 ] || unknown "no *.Tests.ps1 files for Pester"
run pwsh -NoProfile -NonInteractive -Command '
$config = New-PesterConfiguration
$config.Run.Path = "."
$config.Run.Exit = $true
$config.Output.Verbosity = "Detailed"
Invoke-Pester -Configuration $config
'
