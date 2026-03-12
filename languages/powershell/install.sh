#!/usr/bin/env bash
# Install PowerShell
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install powershell; }
install_linux() { apt_install powershell; }

deps() {
    pwsh -Command "if (-not (Get-Module -ListAvailable -Name Pester)) { Install-Module -Name Pester -Force -SkipPublisherCheck -Scope CurrentUser }"
}

log "Installing PowerShell..."
"install_${OS}"
deps
