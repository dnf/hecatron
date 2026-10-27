#!/usr/bin/env bash
# Install Groovy (uses the JDK — delegates to Java)
source "$(dirname "$0")/../_common.sh"

install_macos() {
    if ! has java; then
        bash "$(dirname "$0")/../java/install.sh"
    fi
}

install_linux() {
    if ! has java; then
        bash "$(dirname "$0")/../java/install.sh"
    fi
}

deps() { :; }

log "Installing Groovy..."
"install_${OS}"
deps
