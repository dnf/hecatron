# Smoke test for powershell: build and run a small program in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
pwsh -NoProfile -Command 'Write-Output "hello"; Import-Module Pester; (Get-Module Pester).Version.ToString()'
