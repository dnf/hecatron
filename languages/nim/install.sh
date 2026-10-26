#!/usr/bin/env bash
# Install Nim
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install nim; }
_nim_root="/opt/nim"

# The latest stable official build: distributions lag (Ubuntu 24.04 has Nim
# 1.6, which current Nim code fails to compile with) or have none (Debian 13).
# Nim compiles through the system C compiler.
install_linux() {
    has cc || apt_install build-essential
    has nim || _download_nim
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
