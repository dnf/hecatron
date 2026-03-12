#!/usr/bin/env bash
# Install Bash
source "$(dirname "$0")/../_common.sh"

install_macos() {
    # macOS ships with Bash 3.2; install modern Bash via Homebrew
    brew_install bash
}
install_linux() {
    apt_install bash
}

deps() {
    :
}

log "Installing Bash..."
"install_${OS}"
deps
