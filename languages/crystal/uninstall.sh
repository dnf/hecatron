#!/usr/bin/env bash
# Uninstall Crystal
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall crystal 2>/dev/null || true; }
uninstall_linux() { ${SUDO} apt-get remove -y crystal 2>/dev/null || true; }

log "Uninstalling Crystal..."
"uninstall_${OS}"
