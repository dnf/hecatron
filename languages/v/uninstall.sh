#!/usr/bin/env bash
# Uninstall V
source "$(dirname "$0")/../_common.sh"

uninstall() {
    unlink_into "$HOME/v"
    rm -rf ~/v
}
uninstall_macos() { uninstall; }
uninstall_linux() { uninstall; }

log "Uninstalling V..."
"uninstall_${OS}"
