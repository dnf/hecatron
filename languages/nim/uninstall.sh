#!/usr/bin/env bash
# Uninstall Nim
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall nim 2>/dev/null || true; }
_nim_root="/opt/nim"

uninstall_linux() {
    local tool
    for tool in nim nimble; do
        if [ "$(readlink "/usr/local/bin/${tool}" 2>/dev/null)" = "${_nim_root}/bin/${tool}" ]; then
            $SUDO rm -f "/usr/local/bin/${tool}"
        fi
    done
    $SUDO rm -rf "${_nim_root}"
    $SUDO apt-get remove -y nim 2>/dev/null || true
}

log "Uninstalling Nim..."
"uninstall_${OS}"
