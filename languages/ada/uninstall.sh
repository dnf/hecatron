#!/usr/bin/env bash
# Uninstall Ada toolchain
source "$(dirname "$0")/../_common.sh"

uninstall_macos() {
    rm -rf "$HOME/.local/share/alire/toolchains"/gnat_native* 2>/dev/null || true
    rm -rf "$HOME/.local/share/alire/toolchains"/gprbuild* 2>/dev/null || true
    rm -f "$HOME/.local/bin/alr" 2>/dev/null || true
    log "Removed GNAT, GPRbuild, and Alire."
}

uninstall_linux() { $SUDO apt-get remove -y gnat gprbuild 2>/dev/null || true; }

log "Uninstalling Ada..."
"uninstall_${OS}"
