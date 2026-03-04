#!/usr/bin/env bash
# Uninstall Perl
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { echo "System Perl — not removing."; }
uninstall_linux() { echo "System Perl — not removing."; }

log "Uninstalling Perl..."
"uninstall_${OS}"
