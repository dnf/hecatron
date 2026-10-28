#!/usr/bin/env bash
# Uninstall JavaScript. Its toolchain is Node.js, which TypeScript and Elm
# share (JavaScript's installer is TypeScript's on Linux), so it stays.
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { leave_node; }
uninstall_linux() { leave_node; }

log "Uninstalling JavaScript (Node.js)..."
"uninstall_${OS}"
