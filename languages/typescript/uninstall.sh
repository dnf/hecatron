#!/usr/bin/env bash
# Uninstall TypeScript (Node.js)
source "$(dirname "$0")/../_common.sh"

uninstall_macos() {
    brew uninstall node 2>/dev/null || true
    rm -rf "${HOME}/.local/share/hecatron/node"
}
# install.sh's official builds and their links, and NodeSource's package,
# which earlier versions installed.
uninstall_linux() {
    local link
    for link in /usr/local/bin/*; do
        case "$(readlink "${link}" 2>/dev/null)" in
            /opt/node/*) $SUDO rm -f "${link}" ;;
        esac
    done
    $SUDO rm -rf /opt/node
    $SUDO apt-get remove -y nodejs 2>/dev/null || true
}

log "Uninstalling TypeScript (Node.js)..."
"uninstall_${OS}"
