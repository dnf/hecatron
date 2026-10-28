# The binary's native commands on Windows (docs/design.md §19, #78), on a
# fixture project, with every directory holding bash or Python taken off
# PATH: what phase 1 says runs natively must run without them, and what
# needs bash must say so.
#
#   tests/windows/smoke.ps1 -Binary dist/hct.exe
param([Parameter(Mandatory)] [string] $Binary)

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path "$PSScriptRoot/../..").Path
$hct = (Resolve-Path $Binary).Path
$env:HECATRON_HOME = $repo
$env:HECATRON_OUTPUT = 'agent'

# PATH without bash or Python (git stays: Git for Windows' cmd directory
# holds git.exe alone).
$kept = $env:PATH -split ';' | Where-Object {
    $_ -and -not (Test-Path (Join-Path $_ 'bash.exe')) -and
    -not (Test-Path (Join-Path $_ 'python.exe')) -and
    -not (Test-Path (Join-Path $_ 'python3.exe'))
}
$env:PATH = $kept -join ';'
foreach ($tool in 'bash', 'python', 'python3') {
    if (Get-Command $tool -ErrorAction SilentlyContinue) { throw "$tool is still on PATH" }
}

$failures = @()

# Run hct with arguments; fail unless it exits with one of the statuses
# expected (0 by default) and, when given, prints the pattern.
function Check([string[]] $Arguments, [int[]] $Status = @(0), [string] $Pattern = '') {
    Write-Host "::group::hct $($Arguments -join ' ')"
    $output = & $hct @Arguments 2>&1 | Out-String
    $code = $LASTEXITCODE
    Write-Host $output
    Write-Host "exit $code"
    Write-Host '::endgroup::'
    if ($Status -notcontains $code) {
        $script:failures += "hct $($Arguments -join ' '): exit $code, expected $($Status -join ' or ')"
    } elseif ($Pattern -and $output -notmatch $Pattern) {
        $script:failures += "hct $($Arguments -join ' '): output doesn't match '$Pattern'"
    }
}

# A fixture project: a git repository with a Go and a Python component.
$project = Join-Path ([IO.Path]::GetTempPath()) "hecatron-smoke-$PID"
New-Item -ItemType Directory -Force -Path "$project/svc", "$project/tools" | Out-Null
Set-Content -Path "$project/svc/go.mod" -Value "module example.com/svc`n`ngo 1.22`n" -NoNewline
Set-Content -Path "$project/svc/main.go" -Value "package main`n`nfunc main() {}`n" -NoNewline
Set-Content -Path "$project/tools/pyproject.toml" -Value "[project]`nname = `"tools`"`nversion = `"0.1.0`"`n" -NoNewline
Set-Content -Path "$project/tools/tool.py" -Value "print('hi')`n" -NoNewline
Push-Location $project
try {
    git init -q
    if ($LASTEXITCODE) { throw 'git init failed' }

    Check @('--version') -Pattern '^hecatron \d'
    Check @('help') -Pattern 'Usage'
    Check @('init') -Pattern 'config.json'
    if (-not (Test-Path "$project/.hecatron/config.json")) { $failures += 'init wrote no .hecatron/config.json' }
    Check @('list') -Pattern 'go'
    Check @('check')
    Check @('status') -Pattern 'go'
    Check @('env')
    Check @('env', 'go', '-c', 'svc') -Pattern 'PATH'
    Check @('advisor') -Status 0, 1
    Check @('workflows', '--check') -Status 0, 1
    Check @('dockerfile')
    $skills = Join-Path $project 'skills'
    Check @('skill', 'install', '--dir', $skills) -Pattern 'Installed'
    if (-not (Test-Path "$skills/hecatron/SKILL.md")) { $failures += 'skill install wrote no SKILL.md' }

    # licenses, with a stand-in osv-scanner that reports nothing.
    $stub = Join-Path $project 'stub'
    New-Item -ItemType Directory -Force -Path $stub | Out-Null
    Set-Content -Path "$stub/osv-scanner.cmd" -Value "@echo {`"results`": []}`r`n@exit /b 0`r`n" -NoNewline
    $env:PATH = "$stub;$env:PATH"
    Check @('licenses') -Status 0, 1

    # What needs bash says so, and how to get it.
    Check @('test') -Status 3 -Pattern 'bash'
    Check @('doctor') -Status 3 -Pattern 'bash'
    Check @('install', 'go') -Status 3 -Pattern 'bash'
} finally {
    Pop-Location
}

if ($failures) {
    $failures | ForEach-Object { Write-Host "::error::$_" }
    exit 1
}
Write-Host 'All native commands ran.'
