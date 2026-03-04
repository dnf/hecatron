#!/usr/bin/env bash
# Uninstall Zig
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall zig 2>/dev/null || true; }
uninstall_linux() { $SUDO rm -rf /opt/zig /usr/local/bin/zig; }

log "Uninstalling Zig..."
"uninstall_${OS}"
