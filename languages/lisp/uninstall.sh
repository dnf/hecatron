#!/usr/bin/env bash
# Uninstall SBCL
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall sbcl 2>/dev/null || true; }
uninstall_linux() { $SUDO apt-get remove -y sbcl 2>/dev/null || true; }

log "Uninstalling SBCL..."
"uninstall_${OS}"
