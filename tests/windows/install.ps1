# install.ps1 on a Windows runner, against a published release: first with
# Windows PowerShell 5.1 as a file, then again (an update) with PowerShell 7
# as `irm | iex` runs it. Checks the layout, the checksum-verified binary,
# that the user's PATH gets the directory once, and that a new process
# finds hct and its home. Then the binary built from this commit replaces
# the release's, to check it finds the same home.
#
#   tests/windows/install.ps1 -Binary dist/hct.exe     (HECATRON_VERSION=v0.5.0)
param([Parameter(Mandatory)] [string] $Binary)

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path "$PSScriptRoot/../..").Path
$built = (Resolve-Path $Binary).Path
$root = Join-Path ([IO.Path]::GetTempPath()) "hecatron-install-$PID"
$env:HECATRON_INSTALL_DIR = $root
$bin = Join-Path $root 'home\bin'
$want = ($env:HECATRON_VERSION -replace '^v', '')

function Fail([string] $Message) { Write-Host "::error::$Message"; exit 1 }
function UserPath { (Get-Item HKCU:\Environment).GetValue('Path', '', 'DoNotExpandEnvironmentNames') }

$before = UserPath
try {
    Write-Host '::group::Windows PowerShell 5.1: powershell -File install.ps1'
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $repo 'install.ps1')
    if ($LASTEXITCODE) { Fail "install.ps1 (5.1) exited $LASTEXITCODE" }
    Write-Host '::endgroup::'

    Write-Host '::group::PowerShell 7, again: Get-Content install.ps1 | iex'
    pwsh -NoProfile -Command "Get-Content -Raw '$(Join-Path $repo 'install.ps1')' | Invoke-Expression"
    if ($LASTEXITCODE) { Fail "install.ps1 (7, iex) exited $LASTEXITCODE" }
    Write-Host '::endgroup::'

    foreach ($file in 'hct.exe', 'hecatron.exe') {
        if (-not (Test-Path (Join-Path $bin $file))) { Fail "no $file in $bin" }
    }
    if (Test-Path (Join-Path $bin 'hct')) { Fail "bin\hct (the archive's unpacked link) is still there" }
    $installed = (Get-Content (Join-Path $root 'home\VERSION') -Raw).Trim()
    if ($want -and $installed -ne $want) { Fail "installed VERSION $installed, want $want" }
    if (Test-Path "$root\home.old") { Fail 'the previous install was left behind' }

    $entries = @((UserPath) -split ';' | Where-Object { $_.TrimEnd('\') -eq $bin.TrimEnd('\') })
    if ($entries.Count -ne 1) { Fail "the user's PATH has $bin $($entries.Count) times, want once: $(UserPath)" }

    # A new process with the user's PATH from the registry, as a new
    # terminal gets it.
    $machine = [Environment]::GetEnvironmentVariable('Path', 'Machine')
    $user = [Environment]::GetEnvironmentVariable('Path', 'User')
    $env:PATH = "$user;$machine"
    $version = (pwsh -NoProfile -Command 'hct --version' | Out-String).Trim()
    if ($LASTEXITCODE -or $version -ne $installed) { Fail "hct --version in a new process: '$version' (exit $LASTEXITCODE), want $installed" }
    $help = pwsh -NoProfile -Command 'hct help' | Out-String
    if ($help -notmatch 'Usage') { Fail "hct help: $help" }
    Write-Host "hct $version runs from the user's PATH"

    # This commit's binary in the release's place finds the same home.
    Copy-Item -Force $built (Join-Path $bin 'hct.exe')
    $skill = (& (Join-Path $bin 'hct.exe') skill path | Out-String).Trim()
    $expected = Join-Path $root 'home\plugins\hecatron\skills\hecatron'
    if ($skill -ne $expected) { Fail "hct skill path: '$skill', want '$expected'" }
    Write-Host "This commit's hct.exe finds its home: $skill"
} finally {
    Set-ItemProperty HKCU:\Environment Path $before -Type ExpandString
    Remove-Item -Recurse -Force $root -ErrorAction SilentlyContinue
}
Write-Host 'install.ps1 works.'
exit 0
