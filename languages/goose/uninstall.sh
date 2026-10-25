#!/usr/bin/env bash
# Uninstall Goose
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { ${SUDO} rm -rf /opt/goose /usr/local/bin/goose; }
uninstall_linux() { ${SUDO} rm -rf /opt/goose /usr/local/bin/goose; }

log "Uninstalling Goose..."
"uninstall_${OS}"
