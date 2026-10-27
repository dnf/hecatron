#!/usr/bin/env bash
# Uninstall Dart (and the Flutter its installer may have added)
source "$(dirname "$0")/../_common.sh"

uninstall_macos() {
    brew uninstall dart-sdk 2>/dev/null || true
    brew uninstall --cask flutter 2>/dev/null || true
}
uninstall_linux() {
    $SUDO rm -rf /opt/dart-sdk /opt/flutter
    $SUDO rm -f /usr/local/bin/dart /usr/local/bin/flutter
}

log "Uninstalling Dart..."
"uninstall_${OS}"
