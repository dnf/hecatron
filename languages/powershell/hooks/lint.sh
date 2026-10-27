#!/usr/bin/env bash
# Default lint hook: PSScriptAnalyzer's warnings and errors, with the
# project's PSScriptAnalyzerSettings.psd1 if any, failing on any finding.
source "$(dirname "$0")/../../_hooks.sh"
run pwsh -NoProfile -NonInteractive -Command '
$options = @{ Path = "."; Recurse = $true; Severity = @("Warning", "Error") }
if (Test-Path PSScriptAnalyzerSettings.psd1) { $options.Settings = "PSScriptAnalyzerSettings.psd1" }
$findings = @(Invoke-ScriptAnalyzer @options)
$findings | Format-Table -AutoSize | Out-String -Width 200 | Write-Output
exit [int]($findings.Count -gt 0)
'
