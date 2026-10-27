#!/usr/bin/env bash
# Uninstall Haxe (the haxelib repository in ~/haxelib stays)
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall haxe 2>/dev/null || true; }
uninstall_linux() { $SUDO rm -rf /opt/haxe /usr/local/bin/haxe /usr/local/bin/haxelib; }

log "Uninstalling Haxe..."
"uninstall_${OS}"
