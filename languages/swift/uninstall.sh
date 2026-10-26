#!/usr/bin/env bash
# Uninstall Swift
source "$(dirname "$0")/../_common.sh"

_swift_root="/opt/swift"

uninstall_macos() { echo "Xcode Swift — not removing."; }
uninstall_linux() {
    local tool
    for tool in swift swiftc swift-format sourcekit-lsp; do
        if [ "$(readlink "/usr/local/bin/${tool}" 2>/dev/null)" = "${_swift_root}/usr/bin/${tool}" ]; then
            $SUDO rm -f "/usr/local/bin/${tool}"
        fi
    done
    $SUDO rm -rf "${_swift_root}"
}

log "Uninstalling Swift..."
"uninstall_${OS}"
