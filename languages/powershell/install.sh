#!/usr/bin/env bash
# Install PowerShell
source "$(dirname "$0")/../_common.sh"

_pwsh_root="/opt/microsoft/powershell/7"

install_macos() { brew_install powershell; }

# The official tarball, latest release. Distributions don't package
# PowerShell, Microsoft's apt repository doesn't cover every release or arm64,
# and the tarball works on all of them. It needs ICU at runtime.
install_linux() {
    apt_install libicu-dev
    has pwsh || _download_pwsh
    $SUDO ln -sfn "${_pwsh_root}/pwsh" /usr/local/bin/pwsh
}
_pwsh_arch() {
    case "$(uname -m)" in
        aarch64) echo "arm64" ;;
        *)       echo "x64" ;;
    esac
}
_download_pwsh() {
    local latest version
    latest="$(curl -fsSLI -o /dev/null -w '%{url_effective}' https://github.com/PowerShell/PowerShell/releases/latest)"
    version="${latest##*/v}"
    log "Downloading PowerShell ${version}..."
    $SUDO rm -rf "${_pwsh_root}"
    $SUDO mkdir -p "${_pwsh_root}"
    curl -fsSL "https://github.com/PowerShell/PowerShell/releases/download/v${version}/powershell-${version}-linux-$(_pwsh_arch).tar.gz" |
        $SUDO tar -C "${_pwsh_root}" -xz
    $SUDO chmod +x "${_pwsh_root}/pwsh"
}

# Pester for the default test hook, PSScriptAnalyzer for the lint hook, from
# the PowerShell Gallery. Install-Module reports a failed download without
# failing, so check, and try once more.
_has_module() { pwsh -NoProfile -Command "if (-not (Get-Module -ListAvailable -Name $1)) { exit 1 }"; }
_install_module() {
    pwsh -NoProfile -Command "Install-Module -Name $1 -Force -SkipPublisherCheck -Scope CurrentUser -ErrorAction Stop"
}
deps() {
    local _module
    for _module in Pester PSScriptAnalyzer; do
        _has_module "${_module}" && continue
        _install_module "${_module}" || true
        _has_module "${_module}" && continue
        log "${_module} didn't install; trying again..."
        _install_module "${_module}"
        _has_module "${_module}"
    done
}

log "Installing PowerShell..."
"install_${OS}"
deps
