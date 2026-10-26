#!/usr/bin/env bash
# Uninstall Mojo
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { uv tool uninstall mojo 2>/dev/null || true; }
uninstall_linux() { uv tool uninstall mojo 2>/dev/null || true; }

log "Uninstalling Mojo..."
"uninstall_${OS}"
