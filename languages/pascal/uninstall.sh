#!/usr/bin/env bash
# Uninstall Free Pascal Compiler
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall fpc 2>/dev/null || true; }
uninstall_linux() { $SUDO apt-get remove -y fpc 2>/dev/null || true; }

log "Uninstalling Pascal (Free Pascal Compiler)..."
"uninstall_${OS}"
