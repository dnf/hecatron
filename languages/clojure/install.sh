#!/usr/bin/env bash
# Install Clojure CLI
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install clojure/tools/clojure; }
install_linux() {
    if ! has clojure; then
        curl -fsSL https://download.clojure.org/install/linux-install.sh | $SUDO bash
    fi
}

deps() { :; }

log "Installing Clojure..."
"install_${OS}"
deps
