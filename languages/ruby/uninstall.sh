#!/usr/bin/env bash
# Uninstall Ruby: install.sh's Rubies, their links and ruby-build, and the
# packages earlier versions installed.
source "$(dirname "$0")/../_common.sh"

uninstall_macos() {
    rm -rf "${HOME}/.local/share/hecatron/ruby"
    brew uninstall ruby-build ruby 2>/dev/null || true
}
uninstall_linux() {
    local link
    for link in /usr/local/bin/*; do
        case "$(readlink "${link}" 2>/dev/null)" in
            /opt/ruby/*) $SUDO rm -f "${link}" ;;
        esac
    done
    $SUDO rm -rf /opt/ruby /opt/ruby-build
    $SUDO apt-get remove -y ruby ruby-bundler 2>/dev/null || true
}

log "Uninstalling Ruby..."
"uninstall_${OS}"
