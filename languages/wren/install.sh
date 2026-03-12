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
    log "Installing wren_cli from prebuilt release..."
    apt_install curl unzip
    local tmpdir
    tmpdir="$(mktemp -d)"
    curl -fsSL -o "${tmpdir}/wren-cli.zip" \
        https://github.com/wren-lang/wren-cli/releases/download/0.4.0/wren-cli-linux-0.4.0.zip
    unzip -o "${tmpdir}/wren-cli.zip" -d "${tmpdir}"
    ${SUDO} cp "${tmpdir}/wren-cli-linux-0.4.0/wren_cli" /usr/local/bin/wren_cli
    ${SUDO} chmod +x /usr/local/bin/wren_cli
    rm -rf "${tmpdir}"
}

log "Installing Wren CLI..."
"install_${OS}"
