#!/usr/bin/env bash
# Uninstall Rust
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { rustup self uninstall -y 2>/dev/null || true; }
uninstall_linux() { rustup self uninstall -y 2>/dev/null || true; }

log "Uninstalling Rust..."
"uninstall_${OS}"
