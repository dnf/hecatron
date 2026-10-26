#!/usr/bin/env bash
# Install Odin
source "$(dirname "$0")/../_common.sh"

_odin_root="/opt/odin"

install_macos() { brew_install odin; }

# The latest monthly release for this architecture. Odin links through clang.
install_linux() {
    apt_install clang
    has odin || _download_odin
    $SUDO ln -sfn "${_odin_root}/odin" /usr/local/bin/odin
}
_download_odin() {
    local release arch
    release="$(curl -fsSLI -o /dev/null -w '%{url_effective}' https://github.com/odin-lang/Odin/releases/latest)"
    release="${release##*/}"
    arch="amd64"
    [ "$(uname -m)" = "aarch64" ] && arch="arm64"
    log "Downloading Odin ${release} for linux-${arch}..."
    $SUDO rm -rf "${_odin_root}"
    $SUDO mkdir -p "${_odin_root}"
    curl -fsSL "https://github.com/odin-lang/Odin/releases/download/${release}/odin-linux-${arch}-${release}.tar.gz" |
        $SUDO tar -C "${_odin_root}" --strip-components=1 -xz
}

deps() { :; }

log "Installing Odin..."
"install_${OS}"
deps
