#!/usr/bin/env bash
# Uninstall Wren CLI
source "$(dirname "$0")/../_common.sh"

uninstall_macos() {
    brew uninstall wren-cli 2>/dev/null || true
}
uninstall_linux() {
    $SUDO rm -f /usr/local/bin/wren_cli
}

log "Uninstalling Wren CLI..."
"uninstall_${OS}"
