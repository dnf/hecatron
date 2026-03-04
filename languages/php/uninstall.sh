#!/usr/bin/env bash
# Uninstall PHP
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall php composer 2>/dev/null || true; }
uninstall_linux() {
    # Remove both metapackages and versioned packages (e.g. php8.3-cli)
    $SUDO apt-get remove -y 'php*' 2>/dev/null || true
    $SUDO apt-get autoremove -y 2>/dev/null || true
    $SUDO rm -f /usr/local/bin/composer
}

log "Uninstalling PHP..."
"uninstall_${OS}"
