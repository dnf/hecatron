#!/usr/bin/env bash
# Uninstall Tcl
source "$(dirname "$0")/../_common.sh"

uninstall_macos() {
    brew uninstall tcl-tk 2>/dev/null || true
}
uninstall_linux() {
    $SUDO apt-get remove -y tcl 2>/dev/null || true
}

log "Uninstalling Tcl..."
"uninstall_${OS}"
