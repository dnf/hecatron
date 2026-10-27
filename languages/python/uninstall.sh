#!/usr/bin/env bash
# Uninstall Python
source "$(dirname "$0")/../_common.sh"

uninstall_macos() {
    brew uninstall python uv 2>/dev/null || true
    rm -f "${HOME}/.local/bin/uv" "${HOME}/.local/bin/uvx"
}
uninstall_linux() {
    $SUDO apt-get remove -y python3 python3-venv 2>/dev/null || true
    rm -f "${HOME}/.local/bin/uv" "${HOME}/.local/bin/uvx"
}

log "Uninstalling Python..."
"uninstall_${OS}"
