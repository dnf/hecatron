# Install hecatron on Windows, or update it to the latest release, and put it
# on PATH: the counterpart of install.sh (docs/design.md §19).
#
#   irm https://raw.githubusercontent.com/dnf/hecatron/main/install.ps1 | iex
#
# Installs the release's Go binary, checksum-verified against its
# SHA256SUMS, with the release's source (language definitions, scripts,
# VERSION) around it:
#
#   %LOCALAPPDATA%\hecatron\home          the release's source tree (HECATRON_HOME)
#   %LOCALAPPDATA%\hecatron\home\bin      hct.exe and hecatron.exe, put on the user's PATH
#
# The binary finds its home as the directory above the one it is in, as it
# does in a clone, so nothing else needs setting. %LOCALAPPDATA%\hecatron
# itself is left for what hecatron installs (toolchains).
#
# Settings, as environment variables:
#   HECATRON_VERSION=v0.5.0        a release other than the latest
#   HECATRON_INSTALL_DIR=<dir>     instead of %LOCALAPPDATA%\hecatron
#   HECATRON_REPO=<url>            another copy of https://github.com/dnf/hecatron
#   HECATRON_NO_MODIFY_PATH=1      leave the user's PATH alone
#
# Run it again to update; it never adds PATH twice. It needs PowerShell 5.1
# or later, and nothing else: no bash, Git or Python. The commands that run
# hooks still need bash (Git for Windows) until the definitions are
# declarative (#77); hct says so when they're run without it.

# Everything runs in a script block of its own, so `| iex` leaves nothing
# behind in the caller's session but PATH.
& {
    $ErrorActionPreference = 'Stop'
    $ProgressPreference = 'SilentlyContinue' # Invoke-WebRequest's progress bar slows downloads tenfold on 5.1

    function Say([string] $Message) { Write-Host "hecatron: $Message" }

    # A network request, tried up to three times: a transient failure shouldn't
    # end the install.
    function Invoke-WithRetry([scriptblock] $Request) {
        for ($try = 1; ; $try++) {
            try { return & $Request }
            catch {
                if ($try -ge 3) { throw }
                Say "That failed ($($_.Exception.Message)); trying again in $($try * 5) s..."
                Start-Sleep -Seconds ($try * 5)
            }
        }
    }

    function Get-Arch {
        # Under x64 emulation on arm64, PROCESSOR_ARCHITECTURE says AMD64 and
        # PROCESSOR_ARCHITEW6432 the machine's.
        $machine = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
        switch ($machine) {
            'AMD64' { return 'amd64' }
            'ARM64' { return 'arm64' }
            default { throw "hecatron has no Windows binary for $machine (only x86_64 and arm64)" }
        }
    }

    # The release to install: HECATRON_VERSION, else the latest.
    function Get-Tag([string] $Repo) {
        if ($env:HECATRON_VERSION) { return $env:HECATRON_VERSION }
        $api = $Repo -replace '^https://github\.com/', 'https://api.github.com/repos/'
        $release = Invoke-WithRetry { Invoke-RestMethod -Uri "$api/releases/latest" -Headers @{ 'User-Agent' = 'hecatron-install' } }
        return $release.tag_name
    }

    function Get-File([string] $Url, [string] $OutFile) {
        Invoke-WithRetry { Invoke-WebRequest -Uri $Url -OutFile $OutFile -UseBasicParsing -Headers @{ 'User-Agent' = 'hecatron-install' } } | Out-Null
    }

    # The release's binary, checked against its SHA256SUMS.
    function Get-Binary([string] $Repo, [string] $Tag, [string] $Arch, [string] $Temp) {
        $asset = "hecatron-windows-$Arch.exe"
        $base = "$Repo/releases/download/$Tag"
        Get-File "$base/$asset" (Join-Path $Temp $asset)
        Get-File "$base/SHA256SUMS" (Join-Path $Temp 'SHA256SUMS')
        $want = Get-Content (Join-Path $Temp 'SHA256SUMS') | ForEach-Object {
            $fields = -split $_
            if ($fields.Count -eq 2 -and $fields[1].TrimStart('*') -eq $asset) { $fields[0] }
        } | Select-Object -First 1
        if (-not $want) { throw "$Tag's SHA256SUMS lists no $asset" }
        $got = (Get-FileHash -Algorithm SHA256 -Path (Join-Path $Temp $asset)).Hash
        if ($got -ne $want) { throw "$asset's checksum doesn't match $Tag's SHA256SUMS (got $got, want $want)" }
        return (Join-Path $Temp $asset)
    }

    # The release's source tree, from GitHub's archive of the tag (as install.sh
    # clones it), with the binary in its bin\.
    function New-Home([string] $Repo, [string] $Tag, [string] $Binary, [string] $Temp) {
        $zip = Join-Path $Temp 'source.zip'
        Get-File "$Repo/archive/refs/tags/$Tag.zip" $zip
        $unpacked = Join-Path $Temp 'source'
        Expand-Archive -Path $zip -DestinationPath $unpacked
        $top = @(Get-ChildItem -Path $unpacked -Directory)
        if ($top.Count -ne 1 -or -not (Test-Path (Join-Path $top[0].FullName 'VERSION'))) {
            throw "$Tag's source archive isn't a hecatron tree"
        }
        $tree = $top[0].FullName
        # bin\hct is a symbolic link in git, which the archive unpacks as a
        # text file; hct.exe takes its place.
        Remove-Item -Force -ErrorAction SilentlyContinue (Join-Path $tree 'bin\hct')
        Copy-Item $Binary (Join-Path $tree 'bin\hct.exe')
        Copy-Item $Binary (Join-Path $tree 'bin\hecatron.exe')
        return $tree
    }

    # Replace the installed home with the new one; the old one is moved aside
    # first, so a failure leaves an install that works.
    function Install-Home([string] $New, [string] $Target) {
        $parent = Split-Path -Parent $Target
        New-Item -ItemType Directory -Force -Path $parent | Out-Null
        $old = "$Target.old"
        if (Test-Path $old) { Remove-Item -Recurse -Force $old }
        if (Test-Path $Target) {
            if (-not (Test-Path (Join-Path $Target 'VERSION'))) {
                throw "$Target exists and isn't a hecatron install; move it or set HECATRON_INSTALL_DIR"
            }
            Move-Item $Target $old
        }
        try { Move-Item $New $Target }
        catch {
            if (Test-Path $old) { Move-Item $old $Target }
            throw
        }
        if (Test-Path $old) { Remove-Item -Recurse -Force $old -ErrorAction SilentlyContinue }
    }

    # Add a directory to the user's PATH, once, keeping the registry value's
    # %VARIABLES% unexpanded, and tell running programs it changed.
    function Add-UserPath([string] $Dir) {
        $key = Get-Item -Path 'HKCU:\Environment'
        $current = $key.GetValue('Path', '', 'DoNotExpandEnvironmentNames')
        $entries = @($current -split ';' | Where-Object { $_ })
        $normal = { param($p) [Environment]::ExpandEnvironmentVariables($p).TrimEnd('\').ToLowerInvariant() }
        if ($entries | Where-Object { (& $normal $_) -eq (& $normal $Dir) }) {
            return $false
        }
        $value = (@($Dir) + $entries) -join ';'
        Set-ItemProperty -Path 'HKCU:\Environment' -Name 'Path' -Value $value -Type ExpandString
        # Setting a variable through .NET broadcasts WM_SETTINGCHANGE, so new
        # terminals see the change without signing out.
        [Environment]::SetEnvironmentVariable('HECATRON_INSTALL_PATH_UPDATED', '1', 'User')
        [Environment]::SetEnvironmentVariable('HECATRON_INSTALL_PATH_UPDATED', $null, 'User')
        return $true
    }

    function Install-Hecatron {
        $repo = if ($env:HECATRON_REPO) { $env:HECATRON_REPO.TrimEnd('/') } else { 'https://github.com/dnf/hecatron' }
        $root = if ($env:HECATRON_INSTALL_DIR) { $env:HECATRON_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'hecatron' }
        $target = Join-Path $root 'home'
        $bin = Join-Path $target 'bin'
        $arch = Get-Arch
        $tag = Get-Tag $repo
        Say "Installing hecatron $tag (windows-$arch) into $target..."
        $temp = Join-Path ([IO.Path]::GetTempPath()) ("hecatron-install-" + [Guid]::NewGuid().ToString('N'))
        New-Item -ItemType Directory -Path $temp | Out-Null
        try {
            $binary = Get-Binary $repo $tag $arch $temp
            $new = New-Home $repo $tag $binary $temp
            Install-Home $new $target
        } finally {
            Remove-Item -Recurse -Force $temp -ErrorAction SilentlyContinue
        }

        if ($env:HECATRON_NO_MODIFY_PATH -ne '1') {
            if (Add-UserPath $bin) { Say "Added $bin to your PATH" }
        }
        $sessionPath = $env:PATH -split ';' | ForEach-Object { $_.TrimEnd('\') }
        if ($sessionPath -notcontains $bin.TrimEnd('\')) { $env:PATH = "$bin;$env:PATH" }

        $version = & (Join-Path $bin 'hct.exe') --version
        if ($LASTEXITCODE) { throw "the installed hct.exe doesn't run (exit $LASTEXITCODE)" }
        Say "Installed hecatron $version. New terminals have hct; this one does too if you ran this here."
        Say "Commands that run hooks (hct test, lint, build, doctor, install) need bash on Windows for now:"
        Say "install Git for Windows (https://gitforwindows.org), or use WSL."
        Write-Host ''
        Say 'To uninstall:'
        Write-Host "    Remove-Item -Recurse -Force '$root'"
        if ($env:HECATRON_NO_MODIFY_PATH -ne '1') {
            Write-Host "    # and remove $bin from your user PATH:"
            Write-Host "    `$p = (Get-Item HKCU:\Environment).GetValue('Path', '', 'DoNotExpandEnvironmentNames') -split ';' | Where-Object { `$_ -and `$_.TrimEnd('\') -ne '$($bin.TrimEnd('\'))' }"
            Write-Host "    Set-ItemProperty HKCU:\Environment Path (`$p -join ';') -Type ExpandString"
        }
    }

    Install-Hecatron
}
