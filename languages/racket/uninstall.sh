#!/usr/bin/env bash
# Uninstall Racket
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall minimal-racket 2>/dev/null || true; }
uninstall_linux() { $SUDO apt-get remove -y racket 2>/dev/null || true; }

log "Uninstalling Racket..."
"uninstall_${OS}"
