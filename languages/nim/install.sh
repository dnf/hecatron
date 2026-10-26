#!/usr/bin/env bash
# Install Nim
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install nim; }
_nim_root="/opt/nim"

# The distribution's Nim, else the official build (Debian 13 doesn't package
# it). Nim compiles through the system C compiler.
install_linux() {
    has cc || apt_install build-essential
    has nim && return
    if apt_has nim; then
        apt_install nim
    else
        _download_nim
    fi
}
_download_nim() {
    local version arch tool
    version="$(curl -fsSL https://nim-lang.org/channels/stable)"
    arch="x64"
    [ "$(uname -m)" = "aarch64" ] && arch="arm64"
    log "Downloading Nim ${version} for linux_${arch}..."
    $SUDO rm -rf "${_nim_root}"
    $SUDO mkdir -p "${_nim_root}"
    curl -fsSL "https://nim-lang.org/download/nim-${version}-linux_${arch}.tar.xz" |
        $SUDO tar -C "${_nim_root}" --strip-components=1 -xJ
    for tool in nim nimble; do
        $SUDO ln -sfn "${_nim_root}/bin/${tool}" "/usr/local/bin/${tool}"
    done
}

deps() { :; }

log "Installing Nim..."
"install_${OS}"
deps
