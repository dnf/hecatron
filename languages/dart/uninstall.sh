#!/usr/bin/env bash
# Uninstall Dart
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall dart-sdk 2>/dev/null || true; }
uninstall_linux() { $SUDO rm -rf /opt/dart-sdk && $SUDO rm -f /usr/local/bin/dart; }

log "Uninstalling Dart..."
"uninstall_${OS}"
