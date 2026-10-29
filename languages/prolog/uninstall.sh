#!/usr/bin/env bash
# Uninstall SWI-Prolog, and the PPA install.sh adds on Ubuntu
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall swi-prolog 2>/dev/null || true; }
uninstall_linux() {
    $SUDO apt-get remove -y swi-prolog 2>/dev/null || true
    apt_source_remove swi-prolog
}

log "Uninstalling Prolog (SWI-Prolog)..."
"uninstall_${OS}"
