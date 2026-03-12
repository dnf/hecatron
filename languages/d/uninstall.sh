#!/usr/bin/env bash
# Uninstall D (LDC compiler)
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall ldc 2>/dev/null || true; }
uninstall_linux() { $SUDO apt-get remove -y ldc 2>/dev/null || true; }

log "Uninstalling D (LDC)..."
"uninstall_${OS}"
