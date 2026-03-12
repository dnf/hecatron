#!/usr/bin/env bash
# Uninstall R
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall r 2>/dev/null || true; }
uninstall_linux() { $SUDO apt-get remove -y r-base 2>/dev/null || true; }

log "Uninstalling R..."
"uninstall_${OS}"
