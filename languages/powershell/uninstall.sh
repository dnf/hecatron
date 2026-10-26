#!/usr/bin/env bash
# Uninstall PowerShell
source "$(dirname "$0")/../_common.sh"

_pwsh_root="/opt/microsoft/powershell/7"

uninstall_macos() { brew uninstall powershell 2>/dev/null || true; }
uninstall_linux() {
    if [ "$(readlink /usr/local/bin/pwsh 2>/dev/null)" = "${_pwsh_root}/pwsh" ]; then
        $SUDO rm -f /usr/local/bin/pwsh
    fi
    $SUDO rm -rf "${_pwsh_root}"
}

log "Uninstalling PowerShell..."
"uninstall_${OS}"
