#!/usr/bin/env bash
# Uninstall Gleam
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall gleam 2>/dev/null || true; }
uninstall_linux() { $SUDO rm -f /usr/local/bin/gleam; }

log "Uninstalling Gleam..."
"uninstall_${OS}"
