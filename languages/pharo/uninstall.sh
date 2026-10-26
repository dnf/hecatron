#!/usr/bin/env bash
# Uninstall the Pharo VM installed by install.sh
source "$(dirname "$0")/../_common.sh"

_uninstall() {
    rm -rf "${HOME}/.pharo"
    rm -f "${HOME}/.local/bin/pharo"
}

uninstall_macos() { _uninstall; }
uninstall_linux() { _uninstall; }

log "Uninstalling Pharo..."
"uninstall_${OS}"
