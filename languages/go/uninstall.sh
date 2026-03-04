#!/usr/bin/env bash
# Uninstall Go
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall go 2>/dev/null || true; }
uninstall_linux() { $SUDO rm -rf /usr/local/go; }

log "Uninstalling Go..."
"uninstall_${OS}"
