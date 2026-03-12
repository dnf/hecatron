#!/usr/bin/env bash
# Uninstall Nim
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall nim 2>/dev/null || true; }
uninstall_linux() { $SUDO apt-get remove -y nim 2>/dev/null || true; }

log "Uninstalling Nim..."
"uninstall_${OS}"
