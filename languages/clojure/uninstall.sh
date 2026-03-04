#!/usr/bin/env bash
# Uninstall Clojure CLI
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall clojure/tools/clojure 2>/dev/null || true; }
uninstall_linux() { $SUDO rm -f /usr/local/bin/clojure /usr/local/bin/clj; }

log "Uninstalling Clojure..."
"uninstall_${OS}"
