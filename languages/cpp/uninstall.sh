#!/usr/bin/env bash
# Uninstall C++ toolchain
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall cmake 2>/dev/null || true; }
uninstall_linux() { $SUDO apt-get remove -y cmake 2>/dev/null || true; }

log "Uninstalling C++..."
"uninstall_${OS}"
