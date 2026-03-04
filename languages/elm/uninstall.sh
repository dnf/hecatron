#!/usr/bin/env bash
# Uninstall Elm
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { npm uninstall -g elm elm-test 2>/dev/null || true; }
uninstall_linux() { npm uninstall -g elm elm-test 2>/dev/null || true; }

log "Uninstalling Elm..."
"uninstall_${OS}"
