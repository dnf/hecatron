#!/usr/bin/env bash
# Uninstall Lua
source "$(dirname "$0")/../_common.sh"

uninstall_macos() {
    brew uninstall luarocks lua 2>/dev/null || true
}
uninstall_linux() {
    $SUDO apt-get remove -y luarocks lua5.4 liblua5.4-dev 2>/dev/null || true
}

log "Uninstalling Lua..."
"uninstall_${OS}"
