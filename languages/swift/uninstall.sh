#!/usr/bin/env bash
# Uninstall Swift
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { echo "Xcode Swift — not removing."; }
uninstall_linux() { echo "Manual Swift install — remove from /usr/local manually if needed."; }

log "Uninstalling Swift..."
"uninstall_${OS}"
