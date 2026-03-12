#!/usr/bin/env bash
# Uninstall PowerShell
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall powershell 2>/dev/null || true; }
uninstall_linux() { $SUDO apt-get remove -y powershell 2>/dev/null || true; }

log "Uninstalling PowerShell..."
"uninstall_${OS}"
