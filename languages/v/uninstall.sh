#!/usr/bin/env bash
# Uninstall V
source "$(dirname "$0")/../_common.sh"

uninstall_macos() {
    $SUDO rm -f /usr/local/bin/v
    rm -rf ~/v
}
uninstall_linux() {
    $SUDO rm -f /usr/local/bin/v
    rm -rf ~/v
}

log "Uninstalling V..."
"uninstall_${OS}"
