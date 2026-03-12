#!/usr/bin/env bash
# Install Wren CLI
source "$(dirname "$0")/../_common.sh"

install_macos() {
    brew_install wren-cli
}
install_linux() {
    if has wren_cli; then
        log "wren_cli already installed"
        return
    fi
    log "Building wren_cli from source..."
    apt_install git make gcc
    local tmpdir
    tmpdir="$(mktemp -d)"
    git clone --depth 1 https://github.com/wren-lang/wren-cli.git "${tmpdir}/wren-cli"
    cd "${tmpdir}/wren-cli"
    make
    $SUDO cp bin/wren_cli /usr/local/bin/wren_cli
    rm -rf "${tmpdir}"
}

log "Installing Wren CLI..."
"install_${OS}"
