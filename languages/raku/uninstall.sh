#!/usr/bin/env bash
# Uninstall Raku (Rakudo)
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall rakudo 2>/dev/null || true; }
uninstall_linux() { $SUDO apt-get remove -y rakudo 2>/dev/null || true; }

log "Uninstalling Raku (Rakudo)..."
"uninstall_${OS}"
