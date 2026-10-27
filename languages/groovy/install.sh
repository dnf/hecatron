#!/usr/bin/env bash
# Install Groovy (uses JDK 17 — delegates to Java)
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

deps() { jvm_build_tools groovy; }

log "Installing Groovy (JDK 17)..."
"install_${OS}"
deps
