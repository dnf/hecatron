#!/usr/bin/env bash
# Uninstall Odin
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall odin 2>/dev/null || true; }
uninstall_linux() { $SUDO rm -rf /opt/odin /usr/local/bin/odin; }

log "Uninstalling Odin..."
"uninstall_${OS}"
