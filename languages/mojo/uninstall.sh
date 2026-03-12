#!/usr/bin/env bash
# Uninstall Mojo
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { modular uninstall mojo 2>/dev/null || true; }
uninstall_linux() { modular uninstall mojo 2>/dev/null || true; }

log "Uninstalling Mojo..."
"uninstall_${OS}"
