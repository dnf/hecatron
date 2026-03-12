#!/usr/bin/env bash
# Uninstall GNU Octave
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall octave 2>/dev/null || true; }
uninstall_linux() { $SUDO apt-get remove -y octave 2>/dev/null || true; }

log "Uninstalling Octave..."
"uninstall_${OS}"
