#!/usr/bin/env bash
# Uninstall Elm
source "$(dirname "$0")/../_common.sh"

uninstall() {
    has npm || return 0
    unlink_into "$(npm prefix -g)/bin"
    npm uninstall -g elm elm-test 2>/dev/null || true
}
uninstall_macos() { uninstall; }
uninstall_linux() { uninstall; }

log "Uninstalling Elm..."
"uninstall_${OS}"
