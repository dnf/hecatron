#!/usr/bin/env bash
# Uninstall Crystal
source "$(dirname "$0")/../_common.sh"

_crystal_root="/opt/crystal"

uninstall_macos() { brew uninstall crystal 2>/dev/null || true; }
uninstall_linux() {
    local tool
    for tool in crystal shards; do
        if [ "$(readlink "/usr/local/bin/${tool}" 2>/dev/null)" = "${_crystal_root}/bin/${tool}" ]; then
            ${SUDO} rm -f "/usr/local/bin/${tool}"
        fi
    done
    ${SUDO} rm -rf "${_crystal_root}"
    ${SUDO} apt-get remove -y crystal 2>/dev/null || true
}

log "Uninstalling Crystal..."
"uninstall_${OS}"
