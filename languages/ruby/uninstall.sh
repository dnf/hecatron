#!/usr/bin/env bash
# Uninstall Ruby: install.sh's Rubies, their links and ruby-build, and the
# packages earlier versions installed.
source "$(dirname "$0")/../_common.sh"

uninstall_macos() {
    unlink_into "${HOME}/.local/share/hecatron/ruby"
    rm -rf "${HOME}/.local/share/hecatron/ruby" "${HOME}/.local/share/hecatron/ruby-build"
    brew uninstall ruby-build ruby 2>/dev/null || true
}
uninstall_linux() {
    unlink_into /opt/ruby
    $SUDO rm -rf /opt/ruby /opt/ruby-build
    $SUDO apt-get remove -y ruby ruby-bundler 2>/dev/null || true
}

log "Uninstalling Ruby..."
"uninstall_${OS}"
