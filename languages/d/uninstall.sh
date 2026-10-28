#!/usr/bin/env bash
# Uninstall D (LDC compiler and dub)
source "$(dirname "$0")/../_common.sh"

uninstall_macos() {
    brew uninstall ldc 2>/dev/null || true
    brew uninstall dub 2>/dev/null || true
}
uninstall_linux() { $SUDO apt-get remove -y ldc dub 2>/dev/null || true; }

log "Uninstalling D (LDC)..."
"uninstall_${OS}"
