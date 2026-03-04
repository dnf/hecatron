#!/usr/bin/env bash
# Uninstall COBOL toolchain
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall gnu-cobol 2>/dev/null || true; }
uninstall_linux() { $SUDO apt-get remove -y gnucobol 2>/dev/null || true; }

log "Uninstalling COBOL..."
"uninstall_${OS}"
