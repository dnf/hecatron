#!/usr/bin/env bash
# Uninstall Zig: hecatron's downloads, and where older installers put it.
source "$(dirname "$0")/../_common.sh"

log "Uninstalling Zig..."
toolchain_uninstall zig
if [ "${OS}" = "macos" ]; then
    brew uninstall zig 2>/dev/null || true
else
    $SUDO rm -rf /opt/zig /usr/local/bin/zig
fi
