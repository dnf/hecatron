#!/usr/bin/env bash
# Uninstall Python
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall python uv 2>/dev/null || true; }
uninstall_linux() {
    $SUDO apt-get remove -y python3 python3-venv 2>/dev/null || true
    rm -f "$HOME/.local/bin/uv"
}

log "Uninstalling Python..."
"uninstall_${OS}"
