#!/usr/bin/env bash
# Uninstall Julia
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall julia 2>/dev/null || true; }
uninstall_linux() { $SUDO apt-get remove -y julia 2>/dev/null || true; }

log "Uninstalling Julia..."
"uninstall_${OS}"
