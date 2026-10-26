#!/usr/bin/env bash
# Run the Pester suite for one library.
set -euo pipefail

pwsh -NoProfile -Command '
$config = New-PesterConfiguration
$config.Run.Path = "tests"
$config.Output.Verbosity = "Detailed"
$config.Run.Exit = $true
Invoke-Pester -Configuration $config
'
