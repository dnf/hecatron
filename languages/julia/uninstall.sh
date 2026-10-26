#!/usr/bin/env bash
# Uninstall Julia
source "$(dirname "$0")/../_common.sh"

_julia_root="/opt/julia"

uninstall_macos() { brew uninstall julia 2>/dev/null || true; }
uninstall_linux() {
    if [ "$(readlink /usr/local/bin/julia 2>/dev/null)" = "${_julia_root}/bin/julia" ]; then
        $SUDO rm -f /usr/local/bin/julia
    fi
    $SUDO rm -rf "${_julia_root}"
}

log "Uninstalling Julia..."
"uninstall_${OS}"
