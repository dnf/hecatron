#!/usr/bin/env bash
# Uninstall JavaScript (Node.js) — shares Node.js with TypeScript
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall node 2>/dev/null || true; }
uninstall_linux() { $SUDO apt-get remove -y nodejs 2>/dev/null || true; }

log "Uninstalling JavaScript (Node.js)..."
"uninstall_${OS}"
