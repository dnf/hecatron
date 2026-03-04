#!/usr/bin/env bash
# Uninstall Ruby
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall ruby 2>/dev/null || true; }
uninstall_linux() { $SUDO apt-get remove -y ruby ruby-bundler 2>/dev/null || true; }

log "Uninstalling Ruby..."
"uninstall_${OS}"
