#!/usr/bin/env bash
# Install Wren CLI
source "$(dirname "$0")/../_common.sh"

install_macos() {
    brew_install wren-cli
}
# wren-cli publishes x86_64 Linux builds only; elsewhere, build it from source.
# The 0.4.0 release's source no longer compiles with current GCC (a
# conflicting declaration of write), so build the last commit on main, which
# does and reports 0.4.0.
_WREN_CLI_COMMIT="18553636618a4d33f10af9b5ab92da6431784a8c"
install_linux() {
    if has wren_cli; then
        log "wren_cli already installed"
        return
    fi
    apt_install curl unzip
    local tmpdir
    tmpdir="$(mktemp -d)"
    if [ "$(uname -m)" = "x86_64" ]; then
        log "Installing wren_cli from prebuilt release..."
        curl -fsSL -o "${tmpdir}/wren-cli.zip" \
            https://github.com/wren-lang/wren-cli/releases/download/0.4.0/wren-cli-linux-0.4.0.zip
        unzip -o "${tmpdir}/wren-cli.zip" -d "${tmpdir}"
        ${SUDO} cp "${tmpdir}/wren-cli-linux-0.4.0/wren_cli" /usr/local/bin/wren_cli
    else
        log "Building wren_cli from source for $(uname -m)..."
        { has make && has cc; } || apt_install build-essential
        curl -fsSL "https://github.com/wren-lang/wren-cli/archive/${_WREN_CLI_COMMIT}.tar.gz" | tar -C "${tmpdir}" -xz
        make -C "${tmpdir}/wren-cli-${_WREN_CLI_COMMIT}/projects/make" -j"$(nproc)"
        ${SUDO} cp "${tmpdir}/wren-cli-${_WREN_CLI_COMMIT}/bin/wren_cli" /usr/local/bin/wren_cli
    fi
    ${SUDO} chmod +x /usr/local/bin/wren_cli
    rm -rf "${tmpdir}"
}

log "Installing Wren CLI..."
"install_${OS}"
