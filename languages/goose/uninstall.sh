#!/usr/bin/env bash
# Uninstall Goose: hecatron's toolchains directory's, and /opt/goose, where
# versions before it went.
source "$(dirname "$0")/../_common.sh"

uninstall() {
    toolchain_uninstall goose
    if [ -e /opt/goose ]; then
        unlink_into /opt/goose
        ${SUDO} rm -rf /opt/goose
    fi
}
uninstall_macos() { uninstall; }
uninstall_linux() { uninstall; }

log "Uninstalling Goose..."
"uninstall_${OS}"
