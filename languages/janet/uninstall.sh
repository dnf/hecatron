#!/usr/bin/env bash
# Uninstall Janet
source "$(dirname "$0")/../_common.sh"

uninstall_macos() {
    brew uninstall janet 2>/dev/null || true
}
uninstall_linux() {
    ${SUDO} rm -f /usr/local/bin/janet /usr/local/bin/jpm 2>/dev/null || true
    ${SUDO} rm -rf /usr/local/lib/janet 2>/dev/null || true
    ${SUDO} rm -rf /usr/local/include/janet.h /usr/local/include/janet 2>/dev/null || true
}

log "Uninstalling Janet..."
"uninstall_${OS}"
